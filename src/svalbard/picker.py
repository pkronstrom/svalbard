"""Interactive pack picker data structures and TUI."""

import os
import textwrap
from dataclasses import dataclass, field

import readchar
from rich.console import Console
from rich.live import Live
from rich.text import Text

from svalbard.models import Preset, Source

ROW_GROUP = "group"
ROW_PACK = "pack"
ROW_ITEM = "item"


@dataclass
class PickerPack:
    name: str
    description: str
    display_group: str
    sources: list[Source]

    @property
    def total_size_gb(self) -> float:
        return sum(source.size_gb for source in self.sources)

    def checked_count(self, checked_ids: set[str]) -> int:
        return sum(1 for source in self.sources if source.id in checked_ids)

    def checked_size_gb(self, checked_ids: set[str]) -> float:
        return sum(source.size_gb for source in self.sources if source.id in checked_ids)


@dataclass
class PickerGroup:
    display_group: str
    packs: list[PickerPack] = field(default_factory=list)


def build_picker_tree(
    packs: list[Preset],
    checked_ids: set[str] | None = None,
) -> tuple[list[PickerGroup], set[str]]:
    """Build a grouped picker tree from pack presets.

    Returns (tree, checked_ids) where checked_ids is the global selection state.
    """
    selected_ids = set(checked_ids) if checked_ids else set()
    groups: dict[str, PickerGroup] = {}

    # Narrow initial checked_ids to only sources that actually appear in packs
    all_pack_source_ids: set[str] = set()

    for pack in packs:
        display_group = pack.display_group or "Other"
        group = groups.setdefault(display_group, PickerGroup(display_group=display_group))
        picker_pack = PickerPack(
            name=pack.name,
            description=pack.description,
            display_group=display_group,
            sources=list(pack.sources),
        )
        group.packs.append(picker_pack)
        all_pack_source_ids.update(source.id for source in pack.sources)

    initial_checked = selected_ids & all_pack_source_ids
    tree = sorted(groups.values(), key=lambda group: group.display_group)
    return tree, initial_checked


def _build_rows(
    tree: list[PickerGroup],
    collapsed_groups: dict[str, bool],
    collapsed_packs: dict[str, bool],
    checked_ids: set[str] | None = None,
) -> list[dict]:
    checked = checked_ids or set()
    rows: list[dict] = []
    for group in tree:
        rows.append(
            {
                "kind": ROW_GROUP,
                "display_group": group.display_group,
                "packs": group.packs,
            }
        )
        if collapsed_groups.get(group.display_group, False):
            continue

        for pack in group.packs:
            rows.append(
                {
                    "kind": ROW_PACK,
                    "pack": pack,
                    "checked": pack.checked_count(checked),
                    "total": len(pack.sources),
                }
            )
            if collapsed_packs.get(pack.name, True):
                continue

            for source in pack.sources:
                rows.append(
                    {
                        "kind": ROW_ITEM,
                        "source": source,
                        "pack": pack,
                        "checked": source.id in checked,
                    }
                )
    return rows


def _compute_total(tree: list[PickerGroup], checked_ids: set[str]) -> float:
    seen: set[str] = set()
    total = 0.0
    for group in tree:
        for pack in group.packs:
            for source in pack.sources:
                if source.id in checked_ids and source.id not in seen:
                    seen.add(source.id)
                    total += source.size_gb
    return total


def _is_selectable(row: dict) -> bool:
    return row["kind"] in (ROW_GROUP, ROW_PACK, ROW_ITEM)


def _move_cursor(rows: list[dict], idx: int, direction: int) -> int:
    total = len(rows)
    if total == 0:
        return 0
    for _ in range(total):
        idx = (idx + direction) % total
        if _is_selectable(rows[idx]):
            return idx
    return idx


def _format_size(size_gb: float) -> str:
    if size_gb >= 1:
        return f"{size_gb:.1f} GB"
    return f"{size_gb * 1024:.0f} MB"


def _build_display(
    rows: list[dict],
    cursor: int,
    tree: list[PickerGroup],
    checked_ids: set[str],
    free_gb: float,
    hidden_gb: float,
    scroll_offset: int,
    max_visible: int,
    width: int,
) -> str:
    total_gb = _compute_total(tree, checked_ids) + hidden_gb
    fits = free_gb <= 0 or total_gb <= free_gb

    lines = [
        f"[bold]Svalbard — Pack Picker[/bold]{' ' * 20}{free_gb:.0f} GB free",
        "[dim]" + "─" * width + "[/dim]",
        "",
    ]

    visible_rows = rows[scroll_offset:scroll_offset + max_visible]
    for offset, row in enumerate(visible_rows):
        abs_idx = scroll_offset + offset
        prefix = "▸ " if abs_idx == cursor else "  "

        if row["kind"] == ROW_GROUP:
            has_checked = any(
                pack.checked_count(checked_ids) > 0 for pack in row["packs"]
            )
            style = "[bold]" if has_checked or abs_idx == cursor else "[bold dim]"
            lines.append(f"{prefix}{style}{row['display_group']}[/]")
            continue

        if row["kind"] == ROW_PACK:
            pack = row["pack"]
            checked = row["checked"]
            total = row["total"]
            if checked == total and total > 0:
                mark = "☑"
            elif checked > 0:
                mark = "◐"
            else:
                mark = "[dim]☐[/dim]"
            style = "[white]" if checked > 0 or abs_idx == cursor else "[dim]"
            if abs_idx == cursor:
                style = "[bold white]"
            lines.append(
                f"    {prefix}{mark} {style}{pack.name}[/]  "
                f"[dim]{_format_size(pack.checked_size_gb(checked_ids))}[/dim]"
            )
            continue

        source = row["source"]
        mark = "☑" if row["checked"] else "[dim]☐[/dim]"
        style = "[white]" if row["checked"] or abs_idx == cursor else "[dim]"
        if abs_idx == cursor:
            style = "[bold white]"
        label = source.description or source.id
        max_label = max(12, width - 28)
        if len(label) > max_label:
            label = label[: max_label - 3] + "..."
        lines.append(
            f"        {prefix}{mark} {style}{label}[/]  [dim]{_format_size(source.size_gb)}[/dim]"
        )

    lines.append("")

    if 0 <= cursor < len(rows):
        current = rows[cursor]
        description = ""
        if current["kind"] == ROW_ITEM:
            description = current["source"].description or ""
        elif current["kind"] == ROW_PACK:
            description = current["pack"].description or ""
        if description:
            for line in textwrap.wrap(description, width=max(10, width - 4))[:2]:
                lines.append(f"  [dim italic]{line}[/dim italic]")

    lines.append("[dim]" + "─" * width + "[/dim]")
    if fits:
        lines.append(f"  Total: {total_gb:.1f} / {free_gb:.0f} GB{' ' * 20}[green]✓ fits[/green]")
    else:
        lines.append(
            f"  Total: {total_gb:.1f} / {free_gb:.0f} GB{' ' * 16}"
            f"[red]✗ {total_gb - free_gb:.1f} GB over[/red]"
        )
    lines.append("  [dim]↑↓/jk navigate  SPACE toggle  ENTER expand/collapse  q done[/dim]")
    return "\n".join(lines)


def _get_terminal_height() -> int:
    try:
        return os.get_terminal_size().lines
    except OSError:
        return 24


def _update_scroll(total: int, cursor: int, max_visible: int, offset: int) -> int:
    if cursor < offset + 2:
        offset = max(0, cursor - 2)
    elif cursor >= offset + max_visible - 2:
        offset = min(max(0, total - max_visible), cursor - max_visible + 3)
    return max(0, offset)


def run_picker(
    tree: list[PickerGroup],
    checked_ids: set[str],
    free_gb: float = 0,
    hidden_gb: float = 0,
) -> set[str]:
    """Run the interactive pack picker and return selected source ids."""
    console = Console()
    width = min(100, max(40, console.width - 2))
    collapsed_groups = {group.display_group: False for group in tree}
    collapsed_packs = {pack.name: True for group in tree for pack in group.packs}
    cursor = 0
    scroll_offset = 0

    rows = _build_rows(tree, collapsed_groups, collapsed_packs, checked_ids)
    if rows:
        cursor = _move_cursor(rows, -1, 1)

    with Live("", console=console, refresh_per_second=15, screen=True) as live:
        while True:
            rows = _build_rows(tree, collapsed_groups, collapsed_packs, checked_ids)
            max_visible = max(8, _get_terminal_height() - 10)
            scroll_offset = _update_scroll(len(rows), cursor, max_visible, scroll_offset)
            live.update(
                Text.from_markup(
                    _build_display(
                        rows,
                        cursor,
                        tree,
                        checked_ids,
                        free_gb,
                        hidden_gb,
                        scroll_offset,
                        max_visible,
                        width,
                    )
                )
            )

            key = readchar.readkey()
            if key in (readchar.key.UP, "k"):
                cursor = _move_cursor(rows, cursor, -1)
                continue
            if key in (readchar.key.DOWN, "j"):
                cursor = _move_cursor(rows, cursor, 1)
                continue
            if key in ("q", readchar.key.ESC):
                break
            if key not in (" ", readchar.key.ENTER) or not (0 <= cursor < len(rows)):
                continue

            row = rows[cursor]
            if row["kind"] == ROW_GROUP:
                collapsed_groups[row["display_group"]] = not collapsed_groups.get(
                    row["display_group"], False
                )
                continue

            if row["kind"] == ROW_PACK:
                pack = row["pack"]
                if key == readchar.key.ENTER:
                    collapsed_packs[pack.name] = not collapsed_packs.get(pack.name, True)
                    continue
                pack_ids = {source.id for source in pack.sources}
                if pack_ids <= checked_ids:
                    checked_ids -= pack_ids
                else:
                    checked_ids |= pack_ids
                continue

            source = row["source"]
            if source.id in checked_ids:
                checked_ids.discard(source.id)
            else:
                checked_ids.add(source.id)

    return set(checked_ids)


def _demo_main() -> None:
    """Manual entry point for interactive picker testing."""
    from svalbard.paths import workspace_root as resolve_workspace_root
    from svalbard.presets import list_presets, load_preset

    workspace = resolve_workspace_root()
    packs = []
    for name in list_presets(workspace=workspace):
        try:
            preset = load_preset(name, workspace=workspace)
        except (FileNotFoundError, KeyError, ValueError):
            continue
        if preset.kind == "pack":
            packs.append(preset)

    pre_checked: set[str] = set()
    try:
        pre_checked = {source.id for source in load_preset("finland-128", workspace=workspace).sources}
    except (FileNotFoundError, KeyError, ValueError):
        pass

    tree, initial_checked = build_picker_tree(packs, checked_ids=pre_checked)
    selected = run_picker(tree, initial_checked, free_gb=128)
    print(f"\nSelected {len(selected)} sources:")
    for source_id in sorted(selected):
        print(f"  {source_id}")


if __name__ == "__main__":
    _demo_main()
