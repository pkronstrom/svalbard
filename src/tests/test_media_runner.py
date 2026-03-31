from unittest.mock import Mock, patch


def test_probe_media_url_uses_tools_image():
    from svalbard.media import probe_media_url

    with (
        patch("svalbard.media.has_docker", return_value=True),
        patch("svalbard.media.ensure_tools_image", return_value=True),
        patch("svalbard.media.subprocess.run", return_value=Mock(returncode=0)) as run_mock,
    ):
        assert probe_media_url("https://youtube.com/watch?v=abc") is True

    cmd = run_mock.call_args.args[0]
    assert "svalbard-tools:v1" in cmd
    assert "build-media-zim.py" in " ".join(cmd)


def test_probe_media_url_returns_false_without_docker():
    from svalbard.media import probe_media_url

    with patch("svalbard.media.has_docker", return_value=False):
        assert probe_media_url("https://youtube.com/watch?v=abc") is False
