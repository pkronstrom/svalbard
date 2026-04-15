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

    tree, checked = build_picker_tree(packs=packs)

    assert checked == set()
    assert tree == [
        PickerGroup(
            display_group="Maps",
            packs=[
                PickerPack(
                    name="fi-maps",
                    description="Finnish maps",
                    display_group="Maps",
                    sources=[_source("osm-finland"), _source("nature-reserves")],
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

    tree, _ = build_picker_tree(packs=packs)

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

    tree, _ = build_picker_tree(packs=packs)

    assert len(tree) == 1
    assert tree[0].display_group == "Other"


def test_build_tree_with_preset_prechecks():
    """Sources from a preset are pre-checked in the global set."""
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

    _, checked = build_picker_tree(packs=packs, checked_ids={"kiwix"})

    assert checked == {"kiwix"}


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
                )
            ],
        ),
    ]

    assert _compute_total(tree, {"wikimed", "other"}) == 1.8


def test_build_rows_respects_group_and_pack_collapse_state():
    tree, checked = build_picker_tree(
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
        checked_ids=checked,
    )
    assert [row["kind"] for row in rows] == ["group", "pack", "item", "item"]

    rows = _build_rows(
        tree,
        collapsed_groups={"Tools": True},
        collapsed_packs={"tools": False},
        checked_ids=checked,
    )
    assert [row["kind"] for row in rows] == ["group"]


def test_shared_source_toggle_is_global():
    """Unchecking a source in one pack removes it from all packs."""
    packs = [
        Preset(
            name="engineering",
            description="",
            target_size_gb=4,
            region="",
            kind="pack",
            display_group="Engineering",
            sources=[_source("stackexchange-electronics"), _source("devdocs-c")],
        ),
        Preset(
            name="communications",
            description="",
            target_size_gb=4,
            region="",
            kind="pack",
            display_group="Communications",
            sources=[_source("stackexchange-electronics"), _source("stackexchange-amateur-radio")],
        ),
    ]

    tree, checked = build_picker_tree(
        packs=packs,
        checked_ids={"stackexchange-electronics", "devdocs-c", "stackexchange-amateur-radio"},
    )

    # All three should be checked
    assert checked == {"stackexchange-electronics", "devdocs-c", "stackexchange-amateur-radio"}

    # Simulate unchecking stackexchange-electronics (global operation)
    checked.discard("stackexchange-electronics")

    # Both packs should now show it unchecked
    eng_pack = tree[0].packs[0]  # Engineering
    comm_pack = tree[1].packs[0]  # Communications
    assert eng_pack.checked_count(checked) == 1  # only devdocs-c
    assert comm_pack.checked_count(checked) == 1  # only stackexchange-amateur-radio


def test_run_picker_returns_none_on_quit(monkeypatch):
    import svalbard.picker as picker

    tree, checked = build_picker_tree(
        packs=[
            Preset(
                name="tools",
                description="Core tools",
                target_size_gb=1,
                region="",
                kind="pack",
                display_group="Tools",
                sources=[_source("kiwix")],
            ),
        ]
    )

    class DummyLive:
        def __init__(self, *args, **kwargs):
            pass

        def __enter__(self):
            return self

        def __exit__(self, exc_type, exc, tb):
            return False

        def update(self, *args, **kwargs):
            pass

    monkeypatch.setattr(picker, "Live", DummyLive)
    monkeypatch.setattr(picker.readchar, "readkey", lambda: "q")

    assert picker.run_picker(tree, checked, free_gb=1) is None


def test_run_picker_applies_selection_with_a(monkeypatch):
    import svalbard.picker as picker

    tree, checked = build_picker_tree(
        packs=[
            Preset(
                name="tools",
                description="Core tools",
                target_size_gb=1,
                region="",
                kind="pack",
                display_group="Tools",
                sources=[_source("kiwix")],
            ),
        ]
    )

    class DummyLive:
        def __init__(self, *args, **kwargs):
            pass

        def __enter__(self):
            return self

        def __exit__(self, exc_type, exc, tb):
            return False

        def update(self, *args, **kwargs):
            pass

    keys = iter(["j", " ", "a"])
    monkeypatch.setattr(picker, "Live", DummyLive)
    monkeypatch.setattr(picker.readchar, "readkey", lambda: next(keys))

    assert picker.run_picker(tree, checked, free_gb=1) == {"kiwix"}
