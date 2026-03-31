"""Tests for the shared Docker helper module."""

from unittest.mock import Mock, patch

from svalbard.docker import TOOLS_IMAGE, has_docker, ensure_tools_image, run_container


def test_has_docker_returns_true_when_daemon_running():
    with patch("svalbard.docker.subprocess.run", return_value=Mock(returncode=0)):
        assert has_docker() is True


def test_has_docker_returns_false_when_not_installed():
    with patch("svalbard.docker.subprocess.run", side_effect=FileNotFoundError):
        assert has_docker() is False


def test_has_docker_returns_false_on_timeout():
    from subprocess import TimeoutExpired
    with patch("svalbard.docker.subprocess.run", side_effect=TimeoutExpired("docker", 10)):
        assert has_docker() is False


def test_ensure_tools_image_builds_when_missing():
    inspect_fail = Mock(returncode=1)
    build_ok = Mock(returncode=0)

    with patch("svalbard.docker.subprocess.run", side_effect=[inspect_fail, build_ok]) as run_mock:
        assert ensure_tools_image() is True

    assert run_mock.call_args_list[0].args[0] == ["docker", "image", "inspect", TOOLS_IMAGE]
    assert run_mock.call_args_list[1].args[0][:2] == ["docker", "build"]


def test_ensure_tools_image_skips_build_when_present():
    inspect_ok = Mock(returncode=0)

    with patch("svalbard.docker.subprocess.run", return_value=inspect_ok) as run_mock:
        assert ensure_tools_image() is True

    assert run_mock.call_count == 1


def test_ensure_tools_image_returns_false_when_build_fails():
    inspect_fail = Mock(returncode=1)
    build_fail = Mock(returncode=1)

    with patch("svalbard.docker.subprocess.run", side_effect=[inspect_fail, build_fail]):
        assert ensure_tools_image() is False


def test_run_container_builds_correct_command():
    with patch("svalbard.docker.subprocess.run", return_value=Mock(returncode=0)) as run_mock:
        run_container(
            ["ogr2ogr", "-f", "GPKG", "out.gpkg", "in.shp"],
            mounts={"/host/data": "/data"},
        )

    cmd = run_mock.call_args.args[0]
    assert cmd[:3] == ["docker", "run", "--rm"]
    assert "-v" in cmd
    assert "/host/data:/data" in cmd
    assert TOOLS_IMAGE in cmd
    assert cmd[-5:] == ["ogr2ogr", "-f", "GPKG", "out.gpkg", "in.shp"]
