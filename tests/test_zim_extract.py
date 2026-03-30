"""Tests for ZIM text extraction utilities (pure functions only)."""

from svalbard.zim_extract import strip_html, truncate_text


class TestStripHtml:
    def test_removes_tags(self):
        assert strip_html("<p>Hello <b>world</b></p>") == "Hello world"

    def test_collapses_whitespace(self):
        assert strip_html("Hello   \n\n  world") == "Hello world"

    def test_handles_empty_input(self):
        assert strip_html("") == ""

    def test_decodes_html_entities(self):
        assert strip_html("fish &amp; chips &lt;3") == "fish & chips <3"


class TestTruncateText:
    def test_leaves_short_text_alone(self):
        text = "Hello world."
        assert truncate_text(text, max_chars=100) == text

    def test_truncates_at_sentence_boundary(self):
        text = "First sentence. Second sentence. Third sentence."
        result = truncate_text(text, max_chars=35)
        assert result == "First sentence. Second sentence."

    def test_truncates_at_word_boundary_when_no_sentence(self):
        text = "one two three four five six seven eight nine ten"
        result = truncate_text(text, max_chars=20)
        # Should break at a word boundary and not exceed max_chars
        assert len(result) <= 20
        assert not result.endswith(" ")
        assert result == "one two three four"
