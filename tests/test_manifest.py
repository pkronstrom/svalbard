from svalbard.manifest import Manifest, ManifestEntry


def test_manifest_roundtrip(tmp_path):
    entry = ManifestEntry(
        id="wiki-en",
        type="wiki",
        filename="wiki-en.zim",
        size_bytes=123456,
        platform="",
        tags=["reference", "english"],
        depth="comprehensive",
        downloaded="2026-03-29",
        url="https://example.com/wiki-en.zim",
        checksum_sha256="abc123",
    )
    manifest = Manifest(
        preset="finland-128",
        region="finland",
        target_path="/mnt/drive",
        created="2026-03-29",
        last_synced="2026-03-29",
        entries=[entry],
    )
    path = tmp_path / "manifest.yaml"
    manifest.save(path)

    loaded = Manifest.load(path)
    assert loaded.preset == "finland-128"
    assert loaded.region == "finland"
    assert loaded.target_path == "/mnt/drive"
    assert loaded.created == "2026-03-29"
    assert loaded.last_synced == "2026-03-29"
    assert len(loaded.entries) == 1

    e = loaded.entries[0]
    assert e.id == "wiki-en"
    assert e.type == "wiki"
    assert e.filename == "wiki-en.zim"
    assert e.size_bytes == 123456
    assert e.platform == ""
    assert e.tags == ["reference", "english"]
    assert e.depth == "comprehensive"
    assert e.downloaded == "2026-03-29"
    assert e.url == "https://example.com/wiki-en.zim"
    assert e.checksum_sha256 == "abc123"
    assert "enabled_groups" not in path.read_text()


def test_manifest_exists(tmp_path):
    assert Manifest.exists(tmp_path) is False
    (tmp_path / "manifest.yaml").write_text("preset: test\n")
    assert Manifest.exists(tmp_path) is True


def test_entry_by_id():
    entries = [
        ManifestEntry(id="wiki-en", type="wiki", filename="wiki-en.zim", size_bytes=100),
        ManifestEntry(id="maps-us", type="maps", filename="maps-us.mbt", size_bytes=200),
    ]
    manifest = Manifest(preset="finland-128", region="finland", target_path="/mnt/drive", entries=entries)

    found = manifest.entry_by_id("maps-us")
    assert found is not None
    assert found.id == "maps-us"
    assert found.type == "maps"

    assert manifest.entry_by_id("nonexistent") is None


def test_manifest_roundtrip_preserves_platform(tmp_path):
    manifest = Manifest(
        preset="finland-128",
        region="finland",
        target_path="/mnt/drive",
        entries=[
            ManifestEntry(
                id="kiwix-serve",
                type="binary",
                platform="linux-x86_64",
                filename="kiwix-tools_linux-x86_64.tar.gz",
                size_bytes=123,
            )
        ],
    )
    path = tmp_path / "manifest.yaml"
    manifest.save(path)
    loaded = Manifest.load(path)
    assert loaded.entries[0].platform == "linux-x86_64"
