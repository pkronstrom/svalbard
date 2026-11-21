"""Prepare an offline PlatformIO Core generation from provisioned archives."""
import argparse
import json
import os
import shutil
import subprocess
import sys
from pathlib import Path


def copied_site_packages(source, destination):
    destination.mkdir(parents=True, exist_ok=True)
    for child in source.iterdir():
        target = destination / child.name
        if child.is_dir():
            shutil.copytree(child, target, dirs_exist_ok=True)
        else:
            shutil.copy2(child, target)

def canonical_framework_version(framework):
    from platformio.package.version import get_original_version
    version = str(framework.metadata.version.truncate())
    original = get_original_version(version)
    if original:
        return original
    cmake = Path(framework.path) / "tools" / "cmake" / "version.cmake"
    parts = {}
    if cmake.is_file():
        import re
        for key, value in re.findall(r"set\(IDF_VERSION_(MAJOR|MINOR|PATCH) (\d+)\)", cmake.read_text()):
            parts[key] = value
    if len(parts) == 3:
        return ".".join(parts[key] for key in ("MAJOR", "MINOR", "PATCH"))
    raise RuntimeError("cannot determine original ESP-IDF framework version")


def validate_venv(venv_python):
    validation = r'''
import importlib
import importlib.metadata
import platform
import sys

from packaging.specifiers import SpecifierSet
from packaging.version import Version

deps = {
    "urllib3": "<2",
    "cryptography": "~=46.0.0",
    "pyparsing": ">=3.1.0,<4",
    "idf-component-manager": "~=3.1",
    "esp-idf-kconfig": "~=3.9.0",
    "pydantic": "~=2.12.0",
    # esptool's pyproject.toml direct runtime dependencies
    "bitstring": ">=3.1.6,!=4.2.0",
    "ecdsa": ">=0.16.0",
    "pyserial": ">=3.3",
    "reedsolo": ">=1.5.3,<1.8",
    "PyYAML": ">=5.1",
    "intelhex": "",
}
if sys.platform == "darwin" and "arm" in platform.machine().lower():
    deps["chardet"] = ">=3.0.2,<4"
if sys.platform != "win32":
    deps["argcomplete"] = ">=3"
modules = {
    "urllib3": "urllib3",
    "cryptography": "cryptography",
    "pyparsing": "pyparsing",
    "idf-component-manager": "idf_component_manager",
    "esp-idf-kconfig": "esp_idf_kconfig",
    "pydantic": "pydantic",
    "bitstring": "bitstring",
    "ecdsa": "ecdsa",
    "pyserial": "serial",
    "reedsolo": "reedsolo",
    "PyYAML": "yaml",
    "intelhex": "intelhex",
}
if sys.platform != "win32":
    modules["argcomplete"] = "argcomplete"
for distribution, spec in deps.items():
    found = importlib.metadata.version(distribution)
    if Version(found) not in SpecifierSet(spec):
        raise RuntimeError("%s %s does not satisfy %s" % (distribution, found, spec))
for module in modules.values():
    importlib.import_module(module)
'''
    subprocess.run([str(venv_python), "-c", validation], check=True)


def validate_closure(packages, platform_package):
    from platformio.dependencies import get_core_dependencies
    from platformio.platform.factory import PlatformFactory
    from platformio.package.manager.tool import ToolPackageManager
    from platformio.package.meta import PackageSpec
    provided = {(item["owner"], item["name"]): item for item in packages}
    required = {("platformio", "tool-scons"): get_core_dependencies()["tool-scons"]}
    for board in ("esp32dev", "esp32-s2-saola-1", "esp32-s3-devkitc-1"):
        platform = PlatformFactory.new(platform_package)
        platform.configure_default_packages({
            "board": board,
            "framework": ["espidf"],
            # Espressif32's platform override consumes the project-config name.
            "pioframework": ["espidf"],
        }, [])
        for name, options in platform.packages.items():
            if not options.get("optional"):
                required[(options.get("owner") or "platformio", name)] = options.get("version")
    missing = ["/".join(key) for key in required if key not in provided]
    if missing:
        raise RuntimeError("platformio inventory is missing required packages: " + ", ".join(sorted(missing)))
    manager = ToolPackageManager()
    for (owner, name), requirement in required.items():
        if requirement:
            package = manager.get_package(PackageSpec(owner=owner, name=name, requirements=requirement))
            if not package:
                raise RuntimeError("installed package does not satisfy %s/%s %s" % (owner, name, requirement))


def main():
    parser = argparse.ArgumentParser()
    parser.add_argument("--generation", required=True)
    parser.add_argument("--packages", required=True)
    parser.add_argument("--drive-root", required=True)
    args = parser.parse_args()
    generation = Path(args.generation)
    drive_root = Path(args.drive_root)
    packages = json.loads(Path(args.packages).read_text())
    platform_name = next(item["platform"] for item in packages if item["platform"])
    python_tools = drive_root / "runtime" / "python" / platform_name / "tools"
    sys.path.insert(0, str(python_tools / "platformio-core"))

    from platformio.package.manager.base import BasePackageManager
    from platformio.package.manager.platform import PlatformPackageManager
    from platformio.package.manager.tool import ToolPackageManager
    from platformio.package.meta import PackageSpec
    import platformio

    if platformio.__version__ != "6.2.0":
        raise RuntimeError("offline PlatformIO bootstrap requires Core 6.2.0, got %s" % platformio.__version__)

    original_hook = BasePackageManager.call_pkg_script

    def offline_hook(self, package, event):
        manifest = self.load_manifest(package)
        command = (manifest.get("scripts") or {}).get(event)
        if not command:
            return
        if package.metadata.name != "tool-esptoolpy" or event != "postinstall" or command != "package-postinstall.py":
            raise RuntimeError("offline preparation rejects package hook %r for %s" % (command, package.metadata.name))
        copied_site_packages(python_tools / "espidf-python", Path(package.path) / "_contrib")

    BasePackageManager.call_pkg_script = offline_hook
    os.environ["PLATFORMIO_CORE_DIR"] = str(generation / "core")
    platform_items = [item for item in packages if item["kind"] == "platform"]
    if len(platform_items) != 1:
        raise RuntimeError("offline PlatformIO preparation requires exactly one platform archive")
    ordered = platform_items + [item for item in packages if item["kind"] != "platform"]
    for item in ordered:
        # Core 6.2.0 strips the file:// prefix rather than decoding a URI.
        # Keep literal filesystem characters such as + and spaces intact.
        uri = "file://" + str((drive_root / item["path"]).resolve())
        spec = PackageSpec(owner=item["owner"], name=item["name"], uri=uri)
        manager = PlatformPackageManager() if item["kind"] == "platform" else ToolPackageManager()
        manager.install(spec, skip_dependencies=True)
    BasePackageManager.call_pkg_script = original_hook

    platform_package = PlatformPackageManager().get_package(PackageSpec(owner=platform_items[0]["owner"], name=platform_items[0]["name"]))
    if str(platform_package.metadata.version) != "7.1.3":
        raise RuntimeError("offline PlatformIO bootstrap requires espressif32 7.1.3")
    validate_closure(packages, platform_package)
    framework = ToolPackageManager().get_package(PackageSpec(owner="platformio", name="framework-espidf"))
    if not framework:
        raise RuntimeError("required framework-espidf package was not installed")
    version = canonical_framework_version(framework)
    venv = generation / "core" / "penv" / (".espidf-" + version)
    subprocess.run([sys.executable, "-m", "venv", "--clear", str(venv)], check=True)
    site_packages = next((venv / "lib").glob("python*/site-packages"))
    copied_site_packages(python_tools / "espidf-python", site_packages)
    validate_venv(venv / "bin" / "python")
    esptool = next((generation / "core" / "packages" / "tool-esptoolpy").glob("esptool.py"))
    subprocess.run([str(venv / "bin" / "python"), str(esptool), "version"], check=True)
    python_version = subprocess.check_output(
        [str(venv / "bin" / "python"), "-c", "import sys;print('{0}.{1}.{2}-{3}.{4}'.format(*list(sys.version_info)))"],
        text=True,
    ).strip()
    (venv / "pio-idf-venv.json").write_text(json.dumps({"version": "1.0.0", "python_version": python_version}, indent=2))


if __name__ == "__main__":
    main()
