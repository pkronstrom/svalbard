from svalbard.models import Source
from svalbard.taxonomy import compute_coverage, load_taxonomy


def test_load_taxonomy():
    taxonomy = load_taxonomy()
    assert "survival" in taxonomy
    assert "water" in taxonomy["survival"]
    total_domains = sum(len(domains) for domains in taxonomy.values())
    assert total_domains >= 28


def test_compute_coverage():
    taxonomy = load_taxonomy()
    sources = [
        Source(id="src1", type="zim", tags=["water", "fire-shelter"], depth="comprehensive"),
        Source(id="src2", type="pdf", tags=["water"], depth="overview"),
    ]
    coverage = compute_coverage(sources, taxonomy)
    by_domain = {c.domain: c for c in coverage}

    assert by_domain["water"].score == 45  # 30 + 15
    assert by_domain["water"].sources == ["src1", "src2"]
    assert by_domain["fire-shelter"].score == 30
    assert by_domain["dentistry"].score == 0
    assert by_domain["dentistry"].sources == []


def test_score_caps_at_100():
    taxonomy = load_taxonomy()
    sources = [
        Source(id=f"src{i}", type="zim", tags=["water"], depth="comprehensive")
        for i in range(5)
    ]
    coverage = compute_coverage(sources, taxonomy)
    by_domain = {c.domain: c for c in coverage}

    # 5 * 30 = 150, but should cap at 100
    assert by_domain["water"].score == 100
