from svalbard.models import Preset, Source
from svalbard.picker import _build_rows, _compute_total, PickerGroup, PickerPack, build_picker_tree


def _source(source_id: str, display_group: str = "", size_gb: float = 1.0, **kwargs):
    return Source(
        id=source_id,
        type="zim",
        display_group=display_group,
        size_gb=size_gb,
        **kwargs,
    )


def test_build_tree_from_packs():
    """Packs with display_group form the tree structure."""
    packs = [
        Preset(
            name="fi-maps",
            description="Finnish maps",
            target_size_gb=2,
            region="finland",
            kind="pack",
            display_group="Maps",
            sources=[_source("osm-finland"), _source("nature-reserves")],
        ),
        Preset(
            name="tools-base",
            description="Core tools",
            target_size_gb=1,
            region="",
            kind="pack",
            display_group="Tools",
            sources=[_source("kiwix-serve", size_gb=0.2)],
        ),
    ]

    tree = build_picker_tree(packs=packs)

    assert tree == [
        PickerGroup(
            display_group="Maps",
            packs=[
                PickerPack(
                    name="fi-maps",
                    description="Finnish maps",
                    display_group="Maps",
                    sources=[_source("osm-finland"), _source("nature-reserves")],
                    checked_ids=set(),
                )
            ],
        ),
        PickerGroup(
            display_group="Tools",
            packs=[
                PickerPack(
                    name="tools-base",
                    description="Core tools",
                    display_group="Tools",
                    sources=[_source("kiwix-serve", size_gb=0.2)],
                    checked_ids=set(),
                )
            ],
        ),
    ]


def test_build_tree_groups_packs_by_display_group():
    """Multiple packs with same display_group appear under one group."""
    packs = [
        Preset(
            name="fi-survival",
            description="",
            target_size_gb=5,
            region="finland",
            kind="pack",
            display_group="Survival",
            sources=[_source("wikimed")],
        ),
        Preset(
            name="survival-medical",
            description="",
            target_size_gb=3,
            region="",
            kind="pack",
            display_group="Survival",
            sources=[_source("who-emergency")],
        ),
    ]

    tree = build_picker_tree(packs=packs)

    assert len(tree) == 1
    assert tree[0].display_group == "Survival"
    assert [pack.name for pack in tree[0].packs] == ["fi-survival", "survival-medical"]


def test_build_tree_defaults_missing_display_group_to_other():
    packs = [
        Preset(
            name="misc",
            description="",
            target_size_gb=1,
            region="",
            kind="pack",
            sources=[_source("one")],
        ),
    ]

    tree = build_picker_tree(packs=packs)

    assert len(tree) == 1
    assert tree[0].display_group == "Other"


def test_build_tree_with_preset_prechecks():
    """Sources from a preset are pre-checked."""
    packs = [
        Preset(
            name="tools",
            description="",
            target_size_gb=1,
            region="",
            kind="pack",
            display_group="Tools",
            sources=[_source("kiwix"), _source("cyberchef")],
        ),
    ]

    tree = build_picker_tree(packs=packs, checked_ids={"kiwix"})

    pack = tree[0].packs[0]
    assert pack.checked_ids == {"kiwix"}


def test_compute_total_deduplicates_shared_checked_sources():
    shared = _source("wikimed", size_gb=0.8)
    tree = [
        PickerGroup(
            display_group="A",
            packs=[
                PickerPack(
                    name="p1",
                    description="",
                    display_group="A",
                    sources=[shared, _source("other", size_gb=1.0)],
                    checked_ids={"wikimed", "other"},
                )
            ],
        ),
        PickerGroup(
            display_group="B",
            packs=[
                PickerPack(
                    name="p2",
                    description="",
                    display_group="B",
                    sources=[_source("wikimed", size_gb=0.8)],
                    checked_ids={"wikimed"},
                )
            ],
        ),
    ]

    assert _compute_total(tree) == 1.8


def test_build_rows_respects_group_and_pack_collapse_state():
    tree = build_picker_tree(
        packs=[
            Preset(
                name="tools",
                description="Core tools",
                target_size_gb=1,
                region="",
                kind="pack",
                display_group="Tools",
                sources=[_source("kiwix"), _source("cyberchef")],
            ),
        ],
        checked_ids={"kiwix"},
    )

    rows = _build_rows(
        tree,
        collapsed_groups={"Tools": False},
        collapsed_packs={"tools": False},
    )
    assert [row["kind"] for row in rows] == ["group", "pack", "item", "item"]

    rows = _build_rows(
        tree,
        collapsed_groups={"Tools": True},
        collapsed_packs={"tools": False},
    )
    assert [row["kind"] for row in rows] == ["group"]
