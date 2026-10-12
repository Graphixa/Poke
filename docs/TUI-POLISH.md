# Filter and Compare polish

This change builds on `prototype/filters-compare-favourites` (draft PR #2).

## Browse

Footer order is Enter details → Tab view → F2 filter → F3 compare → F4 favourite → Esc clear / quit. Letter keys remain available for search.

## Filter and sort

The view uses the same full-screen rounded frame, border title, palette and Bubbles help as the existing interface. A highlighted row and chevrons identify the focused field. Available choices appear when there is room; compact terminals retain all three fields, the match count and action keys.

- Up/Down or Tab/Shift+Tab changes the field.
- Left/Right or Space changes its value.
- Enter applies the draft. Esc cancels it. Backspace resets the draft.
- Search is retained, and the draft match count respects it. Zero matches gets an explicit recovery hint.
- Favourites remain grouped first, including when sorting by speed or total.

Actual 100×24 text render: [Filter preview](filter-preview.txt).

## Compare

F3 pins the selected Pokémon. Select another and press F3 to compare. Pressing F3 on the pinned Pokémon in browse unpins it, preserving the existing shortcut.

The comparison uses a full-screen frame with pinned and selected columns, names, Pokédex numbers and coloured types. Six aligned base-stat rows use the same 0–255 scale. Wider screens show bars; compact screens retain the values. Total is separate from the individual stat scale. Higher values receive accent emphasis; the numeric difference remains readable without colour. The difference is **selected minus pinned**, with `=` for a tie and `—` for unavailable data. These are base-stat comparisons, not a battle prediction.

Up/Down changes the opponent within the current filtered list, skipping the pinned species. The pin, query and filters remain fixed. Esc returns to browse on the current opponent; Backspace unpins and returns to browse. The footer stays visible. Native artwork remains suppressed in this text comparison screen.

Actual 100×24 text render: [Compare preview](compare-preview.txt).

## Native graphics repair

The old compositor waited for synchronized-output-end or cursor-show sequences to append Sixel. Bubble Tea v2.0.9 can omit both for hidden-cursor frames when synchronized output is unavailable. The ordinary cell frame could therefore erase restored artwork without a native repaint.

The pinned Bubble Tea renderer flushes a complete `bytes.Buffer` through `io.Copy`; `bytes.Buffer.WriteTo` supplies one output `Write`. The compositor now appends native graphics after that write, including frames without mode markers. This integration contract must be revisited when upgrading Bubble Tea.

Output and pending state share one mutex, preventing cell/native writes from overlapping and stale paints from overtaking clears. One-shot clears are consumed by either flush path. Covering screens clear the old artwork before their cells are drawn, so the clear cannot erase the new screen's text.

Regression tests cover unmarked frames, Compare → browse → dossier → browse repeatedly, subsequent cell writes, concurrent output, and single-use clears. Hidden-cursor and screen-transition tests fail with the original compositor and pass with the replacement. This is deterministic output-level verification; a Windows Terminal Sixel visual check is still required.

## Windows Terminal check

1. Pin Charmander with F3, move to Charizard and press F3. Check the columns, differences and Up/Down opponent controls.
2. Press Esc, Enter, Esc. Confirm the browse artwork returns, then move through neighbouring Pokémon. Repeat twice.
3. Open F2, change type/sort, cancel, then repeat and apply. Confirm cancellation preserves browse state and applying preserves search.
4. Resize while comparing and filtering. At 40×18, values and close controls must remain visible; smaller terminals show a resize prompt.
5. Toggle F4 and reopen the app to check favourite persistence.

No cache format, renderer encoding, artwork download or favourite persistence changes are included.
