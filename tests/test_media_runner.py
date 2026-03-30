from unittest.mock import Mock, patch


def test_ensure_media_image_rebuilds_even_when_tag_exists():
    from svalbard.media import ensure_media_image

    inspect_result = Mock(returncode=0)
    build_result = Mock(returncode=0)

    with patch("svalbard.media.subprocess.run", side_effect=[inspect_result, build_result]) as run_mock:
        assert ensure_media_image() is True

    assert run_mock.call_args_list[0].args[0][:3] == ["docker", "image", "inspect"]
    assert run_mock.call_args_list[1].args[0][:2] == ["docker", "build"]
