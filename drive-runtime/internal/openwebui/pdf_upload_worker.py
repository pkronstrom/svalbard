"""One isolated extraction using the Open WebUI recipe's Python packages."""

import contextlib
import errno
import importlib.metadata
import json
import logging
import mmap
import os
import resource
import sys


MIB = 1024 * 1024
MEMORY_BUDGET = 512 * MIB
SLIM_TEXT_BUDGET = 16 * MIB


class UploadError(Exception):
    pass


def mapped_bytes_darwin():
    # Darwin's VM map includes the shared cache and large reservations.
    # Set the limit above the interpreter's baseline, before any input or
    # third-party modules are read. See struct proc_taskinfo in proc_info.h.
    import ctypes

    proc_pidinfo = ctypes.CDLL("/usr/lib/libproc.dylib").proc_pidinfo
    proc_pidinfo.argtypes = [ctypes.c_int, ctypes.c_int, ctypes.c_uint64, ctypes.c_void_p, ctypes.c_int]
    proc_pidinfo.restype = ctypes.c_int
    task = (ctypes.c_uint64 * 12)()
    proc_pidtaskinfo = 4
    if proc_pidinfo(os.getpid(), proc_pidtaskinfo, 0, task, ctypes.sizeof(task)) != ctypes.sizeof(task) or not task[0]:
        raise RuntimeError("cannot determine interpreter VM size")
    return task[0]


def cap_resource(kind, limit):
    for current in resource.getrlimit(kind):
        if current != resource.RLIM_INFINITY:
            limit = min(limit, current)
    resource.setrlimit(kind, (limit, limit))


def cap_memory(check):
    if sys.platform == "darwin":
        baseline = mapped_bytes_darwin()
    elif sys.platform.startswith("linux"):
        baseline = 0
    else:
        raise RuntimeError("no allocation limit for this platform")
    cap_resource(resource.RLIMIT_AS, baseline + MEMORY_BUDGET)

    if check:
        # Anonymous mappings do not touch memory pages. A platform that
        # accepts the limit but still permits this allocation is unavailable.
        try:
            with mmap.mmap(-1, MEMORY_BUDGET + 32 * MIB):
                pass
        except OSError as exc:
            if exc.errno != errno.ENOMEM:
                raise
        else:
            raise RuntimeError("the kernel did not enforce the allocation limit")


def prevent_external_io(event, _):
    if event in {"socket.connect", "socket.getaddrinfo", "subprocess.Popen", "os.system"}:
        raise PermissionError("document extraction runs offline")


def extract_pdf(filename, budget, page_limit):
    from pypdf import PdfReader, apply_configuration
    from pypdf.errors import LimitReachedError

    # The pinned parser can log a nested form's limit error and return the
    # rest of the page. Reject the whole PDF if any form exceeded a limit.
    form_limit_reached = False

    def record_form_limit(record):
        nonlocal form_limit_reached
        if isinstance(record.args, dict) and isinstance(record.args.get("exception"), (LimitReachedError, RecursionError)):
            form_limit_reached = True
        return True

    logging.getLogger("pypdf._page").addFilter(record_form_limit)
    stream_budget = 16 * MIB
    try:
        with apply_configuration(
            maximum_declared_stream_length=stream_budget,
            array_based_stream_maximum_output_length=stream_budget,
            jbig2_maximum_output_length=stream_budget,
            lzw_maximum_output_length=stream_budget,
            run_length_maximum_output_length=stream_budget,
            zlib_maximum_output_length=stream_budget,
            brotli_maximum_output_length=stream_budget,
            image_maximum_buffer_size=stream_budget,
            stream_decoding_work_maximum_length=2 * stream_budget,
            jbig2dec_binary=None,
        ), open("upload", "rb") as file:
            reader = PdfReader(file)
            if reader.is_encrypted:
                raise UploadError("encrypted")
            if len(reader.pages) > page_limit:
                raise UploadError("pages")
            documents = []
            for page_number, page in enumerate(reader.pages, 1):
                text = (page.extract_text() or "").replace("\x00", "").strip()
                if form_limit_reached:
                    raise UploadError("resources")
                if not text:
                    continue
                content = f"[PDF page {page_number}]\n{text}"
                budget -= len(content.encode("utf-8")) + (2 if documents else 0)
                if budget < 0:
                    raise UploadError("context")
                documents.append({
                    "page_content": content,
                    "metadata": {"source": filename, "page": page_number - 1, "page_label": page_number},
                })
    except UploadError:
        raise
    except (LimitReachedError, MemoryError, RecursionError) as exc:
        raise UploadError("resources") from exc
    except Exception as exc:
        raise UploadError("pdf_read") from exc

    if not documents:
        raise UploadError("scanned")
    return documents


def load_slim(filename, mime_type):
    from fastapi import HTTPException
    from open_webui.retrieval.loaders.main import Loader

    loader = Loader({"rag.content_extraction_engine": "", "rag.file.max_size": 8})
    try:
        # The outer Open WebUI Loader still applies its ftfy cleanup after
        # receiving this response. Invoke exactly the same built-in parser.
        docs = loader._get_loader(filename, mime_type, "upload").load()
    except HTTPException as exc:
        if exc.status_code == 503:
            raise UploadError("unsupported") from exc
        raise

    documents = []
    budget = SLIM_TEXT_BUDGET
    for doc in docs:
        budget -= len(doc.page_content.encode("utf-8")) + 2
        if budget < 0:
            raise UploadError("resources")
        document = {"page_content": doc.page_content, "metadata": {**doc.metadata, "source": filename}}
        documents.append(document)
    return documents


def main():
    mode = sys.argv[1]
    cap_resource(resource.RLIMIT_CPU, 12)
    cap_resource(resource.RLIMIT_FSIZE, SLIM_TEXT_BUDGET)
    cap_resource(resource.RLIMIT_CORE, 0)
    sys.addaudithook(prevent_external_io)
    # Account for trusted native modules in the Darwin VM baseline, then
    # impose a hard memory limit before opening the untrusted file.
    if mode in {"check", "slim"}:
        import chardet
        from lxml import etree
        from open_webui.retrieval.loaders.main import Loader, USE_SLIM

        if not USE_SLIM or not callable(Loader._get_loader) or not callable(chardet.detect) or not callable(etree.HTML):
            raise RuntimeError("the slim attachment loaders are unavailable")
    if mode in {"check", "pdf"}:
        from pypdf import PdfReader, apply_configuration

        if not callable(PdfReader) or not callable(apply_configuration):
            raise RuntimeError("the PDF loader is unavailable")
    cap_memory(mode == "check")
    if mode == "check":
        if importlib.metadata.version("open-webui") != "0.13.5" or importlib.metadata.version("pypdf") != "7.2.0":
            raise RuntimeError("the attachment worker needs the pinned loader versions")
        return {"ready": True}

    filename, mime_type, budget, page_limit = sys.argv[2:]
    try:
        if mode == "pdf":
            documents = extract_pdf(filename, int(budget), int(page_limit))
        else:
            documents = load_slim(filename, mime_type)
    except UploadError as exc:
        return {"error": str(exc)}
    except (MemoryError, RecursionError):
        return {"error": "resources"}
    except Exception:
        return {"error": "read"}
    return {"documents": documents}


if __name__ == "__main__":
    # Package imports and parsers may log to stdout. Reserve stdout for JSON.
    with contextlib.redirect_stdout(sys.stderr):
        result = main()
    json.dump(result, sys.stdout, ensure_ascii=False, separators=(",", ":"))
