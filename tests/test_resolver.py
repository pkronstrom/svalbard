from svalbard.models import Source
from svalbard.resolver import resolve_url


class MockResponse:
    def __init__(self, text):
        self.text = text
        self.status_code = 200

    def raise_for_status(self):
        pass


def test_resolve_static_url():
    source = Source(id="test", type="pdf", url="https://example.com/file.pdf")
    assert resolve_url(source) == "https://example.com/file.pdf"


def test_resolve_url_pattern(monkeypatch):
    html = """
    <a href="wikipedia_en_all_nopic_2025-03.zim">wikipedia_en_all_nopic_2025-03.zim</a>
    <a href="wikipedia_en_all_nopic_2025-09.zim">wikipedia_en_all_nopic_2025-09.zim</a>
    <a href="wikipedia_en_all_nopic_2025-06.zim">wikipedia_en_all_nopic_2025-06.zim</a>
    """

    def mock_get(url, **kwargs):
        return MockResponse(html)

    monkeypatch.setattr("svalbard.resolver.httpx.get", mock_get)

    source = Source(
        id="wikipedia",
        type="zim",
        url_pattern="https://download.kiwix.org/zim/wikipedia/wikipedia_en_all_nopic_{date}.zim",
    )
    result = resolve_url(source)
    assert result == "https://download.kiwix.org/zim/wikipedia/wikipedia_en_all_nopic_2025-09.zim"
