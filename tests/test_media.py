from importlib.util import module_from_spec, spec_from_file_location
from pathlib import Path
import sys


def _load_media_builder_module():
    module_path = Path(__file__).resolve().parent.parent / "docker" / "scripts" / "build-media-zim.py"
    spec = spec_from_file_location("build_media_zim", module_path)
    module = module_from_spec(spec)
    assert spec is not None
    assert spec.loader is not None
    sys.modules[spec.name] = module
    spec.loader.exec_module(module)
    return module


def test_yt_dlp_download_cmd_ignores_subtitle_failures_and_skips_translated_subs():
    builder = _load_media_builder_module()

    cmd = builder._yt_dlp_download_cmd("https://www.youtube.com/watch?v=abc", "720p", audio_only=False)

    assert "--ignore-errors" in cmd
    assert "--extractor-args" in cmd
    assert "youtube:skip=translated_subs" in cmd
    assert "--write-subs" in cmd
    assert "--write-auto-subs" in cmd
    sub_langs_index = cmd.index("--sub-langs") + 1
    assert cmd[sub_langs_index] == "en,fi,en-orig,fi-FI"
