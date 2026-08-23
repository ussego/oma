# qs.Ui component reference

Generated from the installed shell source (`/usr/share/omarchy/shell/Ui/`).
Members below are declared on each component's root object. All components
live under `import qs.Ui`; theme values come from `qs.Commons`. Defaults
shown are the shipped values - they track the active Omarchy theme.

## BarIconButton

**Properties**
- `iconComponent` (Component) — default `null`
- `slotSize` (real) — default `Style.bar.iconSlot`
- `opticalSize` (real) — default `Style.bar.iconCanvas`
- `debugOpticalBounds` (bool) — default `Quickshell.env("OMARCHY_DEBUG_BAR_ICONS") === "1"`
- `opticalCenterErrorX` (real) — default `glyph.visible ? glyph.paintedCenterX - opticalCanvas.widt...`
- `glyphPaintedWidth` (real) — default `glyph.visible ? glyph.tightWidth : 0`
- `glyphBaselineY` (real) — default `glyph.visible ? glyph.baselineY : 0`
- `glyphFontSize` (int) — default `glyph.visible ? glyph.renderedFontSize : 0`

## BarIndicator

**Properties**
- `moduleName` (string) — default `""`
- `settings` (var) — default `({})`
- `activeText` (string) — default `""`
- `inactiveText` (string) — default `activeText`
- `activeTooltipText` (string) — default `""`
- `inactiveTooltipText` (string) — default `activeTooltipText`
- `indicatorBlock` (string) — default `"single"`
- `indicatorHost` (var) — default `null`
- `activeOverride` (var) — default `null`
- `effectiveActive` (bool) — default `activeOverride === null || activeOverride === undefined ?...`
- `belongsInBlock` (bool) — default `indicatorBlock === "active" ? effectiveActive : (indicato...`
- `inactiveRevealed` (bool) — default `!effectiveActive && !!indicatorHost && indicatorHost.reve...`

**Functions**
- `extractData(raw)` (function)
- `syncIndicatorOpacity()` (function)

## BarWidget

**Properties**
- `bar` (QtObject) — default `null`
- `moduleName` (string) — default `""`
- `settings` (var) — default `({})`
- `vertical` (bool) — default `bar ? bar.vertical : false`
- `barSize` (int) — default `bar ? bar.barSize : Style.bar.sizeHorizontal`

**Functions**
- `broadcast(method)` (function)
- `setting(name, fallback)` (function)

## BorderOverlay

**Properties**
- `borderSpec` (var) — default `Border.none()`
- `radius` (real) — default `0`
- `_widths` (var) — default `borderSpec && borderSpec.widths ? borderSpec.widths : Geo...`
- `hasBorder` (bool) — default `Geometry.maxWidth(_widths) > 0`
- `_gradient` (var) — default `borderSpec && borderSpec.gradient ? borderSpec.gradient :...`
- `_colors` (var) — default `_gradient.enabled ? _gradient.colors : [Border.color(bord...`
- `_endpoints` (var) — default `Geometry.gradientEndpoints(width, height, _gradient.angle...`
- `_path` (string) — default `Geometry.ringPath(width, height, radius, _widths)`

## BorderSurface

**Properties**
- `borderSpec` (var) — default `Border.none()`
- `padding` (real) — default `0`
- `topPadding` (real) — default `padding`
- `rightPadding` (real) — default `padding`
- `bottomPadding` (real) — default `padding`
- `leftPadding` (real) — default `padding`
- `borderTop` (real) — default `Border.top(borderSpec)`
- `borderRight` (real) — default `Border.right(borderSpec)`
- `borderBottom` (real) — default `Border.bottom(borderSpec)`
- `borderLeft` (real) — default `Border.left(borderSpec)`
- `contentTopInset` (real) — default `borderTop + topPadding`
- `contentRightInset` (real) — default `borderRight + rightPadding`
- `contentBottomInset` (real) — default `borderBottom + bottomPadding`
- `contentLeftInset` (real) — default `borderLeft + leftPadding`
- `usesOverlayBorder` (bool) — default `Border.needsOverlay(borderSpec)`

## Button

**Signals**
- `clicked()`
- `rightClicked()`
- `hovered(bool isHovered)`

**Properties**
- `text` (string) — default `""`
- `iconText` (string) — default `""`
- `tooltipText` (string) — default `""`
- `selected` (bool) — default `false`
- `active` (bool) — default `false`
- `hasCursor` (bool) — default `false`
- `focusable` (bool) — default `false`
- `bordered` (bool) — default `false`
- `foreground` (color) — default `Color.foreground`
- `background` (color) — default `"transparent"`
- `accent` (color) — default `Color.accent`
- `fontFamily` (string) — default `Style.font.family`
- `fontSize` (real) — default `Style.font.body`
- `iconSize` (real) — default `Style.font.icon`
- `iconRotation` (real) — default `0`
- `iconSpinning` (bool) — default `false`
- `horizontalPadding` (real) — default `Style.spacing.controlPaddingX`
- `verticalPadding` (real) — default `Style.spacing.controlPaddingY`
- `leftAlign` (bool) — default `false`
- `tooltipBackground` (color) — default `Color.tooltip.background`
- `tooltipForeground` (color) — default `Color.tooltip.text`
- `tooltipBorder` (color) — default `Color.tooltip.border`
- `hot` (bool) — default `mouseArea.containsMouse || hasCursor`
- `_showFocusRing` (bool) — default `focusable && activeFocus`
- `_selectedColor` (color) — default `Style.selectedStateColor(root.foreground, root.accent)`
- `_tooltipBorderSpec` (var) — default `Border.localOrSurfaceSpec("tooltip", "border", root.toolt...`
- `_focusBorderSpec` (var) — default `Border.controlSpec("focus", root.foreground, root.accent)`
- `_hoverBorderSpec` (var) — default `Border.controlSpec("hover-cursor", root.foreground, root....`
- `_selectedBorderSpec` (var) — default `Border.controlSpec("selected", root.foreground, root.accent)`
- `_normalBorderSpec` (var) — default `Border.controlSpec("normal", root.foreground, root.accent)`
- `_reservedBorderTop` (real) — default `Math.max(`
- `_reservedBorderRight` (real) — default `Math.max(`
- `_reservedBorderBottom` (real) — default `Math.max(`
- `_reservedBorderLeft` (real) — default `Math.max(`
- `_reservedContentLeftInset` (real) — default `_reservedBorderLeft + leftPadding`
- `_borderSpec` (var) — default `_showFocusRing ? _focusBorderSpec`

## ButtonGroup

**Signals**
- `changed(string value)`
- `hovered(int index, bool isHovered)`

**Properties**
- `options` (var) — default `[]`
- `value` (string) — default `""`
- `foreground` (color) — default `Color.foreground`
- `background` (color) — default `Color.background`
- `accent` (color) — default `Color.accent`
- `fontFamily` (string) — default `Style.font.family`
- `fontSize` (real) — default `Style.font.body`
- `focusable` (bool) — default `true`
- `cursorIndex` (int) — default `-1`
- `_focusedIndex` (int) — default `-1`

**Functions**
- `optionValue(o)` (function)
- `optionLabel(o)` (function)
- `optionIcon(o)` (function)
- `optionTooltip(o)` (function)
- `selectedOptionIndex()` (function)
- `activateFocused()` (function)

## ConfirmDialog

**Signals**
- `canceled()`
- `confirmed()`

**Properties**
- `opened` (bool) — default `false`
- `message` (string) — default `""`
- `cancelText` (string) — default `"Cancel"`
- `confirmText` (string) — default `"Confirm"`
- `selectedIndex` (int) — default `1`
- `background` (color) — default `Color.background`
- `foreground` (color) — default `Color.foreground`
- `scrim` (color) — default `Util.alpha(Color.background, 0.7)`
- `selectedBackground` (color) — default `Util.alpha(Color.foreground, 0.08)`
- `selectedText` (color) — default `Color.accent`
- `fontFamily` (string) — default `Style.font.family`
- `cornerRadius` (int) — default `Style.cornerRadius`

**Functions**
- `handleKey(event)` (function)

## CursorSurface

**Properties**
- `hasCursor` (bool) — default `false`
- `current` (bool) — default `false`
- `outline` (bool) — default `false`
- `bordered` (bool) — default `false`
- `foreground` (color) — default `Color.foreground`
- `accent` (color) — default `Color.accent`
- `fill` (color) — default `Style.hoverFillFor(foreground, accent)`
- `currentFill` (color) — default `Style.selectedFillFor(foreground, accent)`

## Dropdown

**Signals**
- `changed(string value)`
- `hovered(bool isHovered)`

**Properties**
- `label` (string) — default `""`
- `value` (string) — default `""`
- `options` (var) — default `[]`
- `foreground` (color) — default `Color.popups.text`
- `background` (color) — default `Color.popups.background`
- `popupBorder` (color) — default `Color.popups.border`
- `accent` (color) — default `Color.accent`
- `popupBorderSpec` (var) — default `Border.localOrSurfaceSpec("popups", "border", popupBorder...`
- `fontFamily` (string) — default `Style.font.family`
- `rowHeight` (int) — default `Style.spacing.controlHeight`
- `popupRowHeight` (int) — default `Style.spacing.popupRowHeight`
- `showLabel` (bool) — default `true`
- `hasCursor` (bool) — default `false`
- `popupOpen` (bool) — default `popup.opened`

**Functions**
- `open() { popup.open() }` (function)
- `close() { popup.close() }` (function)
- `toggle() { popup.opened ? popup.close() : popup.open() }` (function)
- `optionValue(o)` (function)
- `optionLabel(o)` (function)
- `currentLabel()` (function)

## KeyboardPanel

**Properties**
- `owner` (var) — default `null`
- `margin` (int) — default `Style.gapsOut`
- `padding` (int) — default `Style.spacing.popupPadding`
- `contentWidth` (int) — default `Style.space(280)`
- `contentHeight` (int) — default `Style.space(200)`
- `borderSpec` (var) — default `Border.surfaceSpec("popups", "border", Color.popups.borde...`
- `centerOnBar` (bool) — default `false`
- `open` (bool) — default `false`
- `gap` (int) — default `Style.gapsOut`
- `popoutSwitching` (bool) — default `false`
- `popoutSwitchClosing` (bool) — default `false`
- `focusPrimed` (bool) — default `false`
- `focusTarget` (Item) — default `null`
- `coordinatorKey` (var) — default `owner || root`
- `anchorWindow` (var) — default `anchorItem ? anchorItem.QsWindow.window : null`
- `barPos` (string) — default `bar ? bar.position : "top"`
- `_barStripSize` (real) — default `{`
- `anchorScreenPos` (point) — default `{`
- `anchorW` (real) — default `anchorItem ? anchorItem.width : 0`
- `anchorH` (real) — default `anchorItem ? anchorItem.height : 0`
- `screenW` (real) — default `screen ? screen.width : 0`
- `screenH` (real) — default `screen ? screen.height : 0`
- `availableCardWidth` (real) — default `screenW > 0`
- `availableCardHeight` (real) — default `screenH > 0`
- `verticalContentInset` (real) — default `padding * 2 + Border.top(borderSpec) + Border.bottom(bord...`
- `barW` (real) — default `anchorWindow ? anchorWindow.width : screenW`
- `barH` (real) — default `anchorWindow ? anchorWindow.height : 0`
- `cardOrigin` (point) — default `{`

**Functions**
- `close()` (function)
- `beginFocusPrime()` (function)
- `fittedContentWidth(width, cap)` (function)
- `fittedContentHeight(implicitHeight, cap)` (function)
- `cappedContentHeight(height)` (function)

## MultiSelect

**Signals**
- `changed(var values)`
- `hovered(bool isHovered)`

**Properties**
- `label` (string) — default `""`
- `values` (var) — default `[]`
- `options` (var) — default `[]`
- `optionsCommand` (var) — default `[]`
- `optionsCommandCwd` (string) — default `""`
- `placeholderText` (string) — default `"Search..."`
- `emptyText` (string) — default `"No options"`
- `noSelectionText` (string) — default `"None selected"`
- `triggerLabel` (string) — default `""`
- `showLabel` (bool) — default `true`
- `foreground` (color) — default `Color.popups.text`
- `background` (color) — default `Color.popups.background`
- `popupBorder` (color) — default `Color.popups.border`
- `accent` (color) — default `Color.accent`
- `popupBorderSpec` (var) — default `Border.localOrSurfaceSpec("popups", "border", popupBorder...`
- `fontFamily` (string) — default `Style.font.family`
- `rowHeight` (int) — default `Style.spacing.controlHeight`
- `popupRowHeight` (int) — default `Style.spacing.popupRowHeight`
- `popupMinHeight` (int) — default `Style.spacing.searchablePopupMinHeight`
- `hasCursor` (bool) — default `false`
- `popupOpen` (bool) — default `popup.opened`
- `resolvedOptions` (var) — default `[]`
- `loadingOptions` (bool) — default `false`
- `optionsError` (string) — default `""`
- `filtered` (var) — default `resolvedOptions`
- `refreshSeq` (int) — default `0`
- `refreshTimeoutMs` (int) — default `6000`

**Functions**
- `open() { popup.open() }` (function)
- `close() { popup.close() }` (function)
- `toggle() { popup.opened ? popup.close() : popup.open() }` (function)
- `normalizeOption(o)` (function)
- `arrayFrom(v)` (function)
- `normalizeAll(arr)` (function)
- `valueSet()` (function)
- `isSelected(value)` (function)
- `toggleValue(value)` (function)
- `selectionLabel()` (function)
- `recomputeFiltered()` (function)
- `rebuildFromStatic()` (function)
- `parseCommandOutput(text)` (function)
- `refresh()` (function)

## NumberField

**Signals**
- `modified(int value)`
- `hovered(bool on)`

**Properties**
- `label` (string) — default `""`
- `value` (int) — default `0`
- `from` (int) — default `0`
- `to` (int) — default `100`
- `stepSize` (int) — default `1`
- `foreground` (color) — default `Color.foreground`
- `accent` (color) — default `Color.accent`
- `fontFamily` (string) — default `Style.font.family`
- `fontSize` (real) — default `Style.font.body`
- `fieldWidth` (real) — default `Style.spacing.numberFieldWidth`
- `hasCursor` (bool) — default `false`
- `_hovered` (bool) — default `false`
- `field` (alias) — default `spin`

## OpticalGlyph

**Properties**
- `text` (string) — default `""`
- `fontFamily` (string) — default `Style.font.family`
- `fontSize` (real) — default `Style.font.body`
- `color` (color) — default `Color.foreground`
- `debugBounds` (bool) — default `false`
- `renderedFontSize` (int) — default `Math.max(1, Math.round(fontSize))`
- `tightWidth` (real) — default `Math.max(1, glyphMetrics.tightBoundingRect.width)`
- `horizontalCorrection` (real) — default `glyph.implicitWidth / 2 - (glyphMetrics.tightBoundingRect...`
- `paintedCenterX` (real) — default `glyph.x + glyphMetrics.tightBoundingRect.x + tightWidth / 2`
- `baselineY` (real) — default `glyph.y + glyph.baselineOffset`

## Panel

**Properties**
- `bar` (QtObject) — default `null`
- `moduleName` (string) — default `""`
- `settings` (var) — default `({})`
- `ipcTarget` (string) — default `""`
- `manageIpc` (bool) — default `true`
- `controller` (alias) — default `panelController`
- `popoutSwitching` (bool) — default `false`
- `popoutSwitchClosing` (bool) — default `false`
- `opened` (bool) — default `panelController.open`
- `barForeground` (color) — default `bar ? bar.barForeground : Color.foreground`

**Functions**
- `open() { panelController.show() }` (function)
- `close() { panelController.hide() }` (function)
- `closeForPopoutSwitch()` (function)
- `toggle() { opened ? close() : open() }` (function)
- `switchPanel(direction)` (function)
- `setting(name, fallback)` (function)

## PanelActionButton

**Signals**
- `clicked()`
- `hovered(bool isHovered)`

**Properties**
- `iconText` (string) — default `""`
- `tooltipText` (string) — default `""`
- `foreground` (color) — default `Color.foreground`
- `hoverColor` (color) — default `foreground`
- `fontFamily` (string) — default `Style.font.family`
- `fontSize` (real) — default `Style.font.icon`
- `size` (real) — default `Math.max(Style.space(22), fontSize + Style.spacing.sm * 2)`
- `focusable` (bool) — default `false`
- `hasCursor` (bool) — default `false`
- `bordered` (bool) — default `false`
- `_showFocusRing` (bool) — default `focusable && activeFocus`
- `_hot` (bool) — default `(mouse.containsMouse || root.hasCursor) && root.enabled`
- `_borderSpec` (var) — default `_showFocusRing`

## PanelController

**Properties**
- `open` (bool) — default `false`

**Functions**
- `toggle() { open = !open }` (function)
- `show() { if (!open) open = true }` (function)
- `hide() { open = false }` (function)

## PanelHero

**Properties**
- `iconComponent` (Component) — default `null`
- `title` (string) — default `""`
- `meta` (string) — default `""`
- `detail` (string) — default `""`
- `foreground` (color) — default `Color.foreground`
- `fontFamily` (string) — default `Style.font.family`
- `iconSize` (real) — default `Style.font.display`
- `iconOpacity` (real) — default `1.0`
- `metaOpacity` (alias) — default `metaText.opacity`
- `trailingControl` (Component) — default `null`
- `dim` (color) — default `Qt.darker(foreground, 1.4)`
- `trailingInset` (real) — default `trailingLoader.item && trailingLoader.item.visible ? trai...`

## PanelKeyCatcher

**Signals**
- `moveRequested(int dx, int dy)`
- `activateRequested()`
- `returnRequested()`
- `closeRequested()`
- `deleteRequested()`
- `tabRequested(int direction)`
- `textKey(string text)`

**Properties**
- `blocked` (bool) — default `false`

## PanelSectionHeader

**Properties**
- `foreground` (color) — default `Color.foreground`
- `fontFamily` (string) — default `Style.font.family`
- `fontSize` (real) — default `Style.font.caption`

## PanelSeparator

**Properties**
- `foreground` (color) — default `Color.foreground`
- `strength` (real) — default `0.12`

## PanelSlider

**Signals**
- `moved(real value)`
- `released(real value)`
- `rightClicked()`

**Properties**
- `bar` (QtObject) — default `null`
- `value` (real) — default `0`
- `minimum` (real) — default `0`
- `maximum` (real) — default `1`
- `step` (real) — default `0.05`
- `integer` (bool) — default `false`
- `trackColor` (color) — default `bar ? Style.selectedFillFor(bar.foreground, Color.accent)...`
- `fillColor` (color) — default `bar ? bar.foreground : Color.foreground`
- `knobColor` (color) — default `bar ? bar.foreground : Color.foreground`
- `dragging` (bool) — default `false`
- `trackHeight` (real) — default `Math.max(4, Math.round(Style.spacing.controlHeight * 0.11))`
- `knobSize` (real) — default `Math.max(14, Math.round(Style.spacing.controlHeight * 0.38))`
- `liveValue` (real) — default `value`
- `tickCount` (int) — default `0`
- `tickColor` (color) — default `bar ? bar.background : Color.background`
- `range` (real) — default `Math.max(0.0001, maximum - minimum)`
- `progress` (real) — default `Math.max(0, Math.min(1, (liveValue - minimum) / range))`
- `_hot` (bool) — default `mouseArea.containsMouse || root.dragging`

## PanelToolTip

**Properties**
- `panelForeground` (color) — default `Color.tooltip.text`
- `panelBackground` (color) — default `Color.tooltip.background`
- `panelBorder` (color) — default `Color.tooltip.border`
- `fontFamily` (string) — default `Style.font.family`
- `fontSize` (real) — default `Style.font.bodySmall`
- `panelBorderSpec` (var) — default `Border.localOrSurfaceSpec("tooltip", "border", panelBorde...`

## PointerMoveGate

**Properties**
- `referenceItem` (Item) — default `null`
- `threshold` (real) — default `1`
- `primed` (bool) — default `false`
- `initialSampleAllowed` (bool) — default `false`
- `lastX` (real) — default `0`
- `lastY` (real) — default `0`

**Functions**
- `reset()` (function)
- `allowInitialSample()` (function)
- `moved(item, mouse)` (function)

## PopupCard

**Properties**
- `owner` (var) — default `null`
- `margin` (int) — default `Style.gapsOut`
- `padding` (int) — default `Style.spacing.popupPadding`
- `contentWidth` (int) — default `Style.space(280)`
- `contentHeight` (int) — default `Style.space(200)`
- `borderColor` (color) — default `Color.popups.border`
- `borderSpec` (var) — default `Border.localOrSurfaceSpec("popups", "border", borderColor...`
- `open` (bool) — default `false`
- `centerOnBar` (bool) — default `false`
- `triggerMode` (string) — default `"click"`
- `coordinatorKey` (var) — default `owner || root`
- `anchorWindow` (var) — default `anchorItem ? anchorItem.QsWindow.window : null`
- `popupScreen` (var) — default `anchorWindow ? anchorWindow.screen : null`
- `containsMouse` (bool) — default `cardHover.hovered`
- `screenW` (real) — default `popupScreen ? popupScreen.width : 0`
- `screenH` (real) — default `popupScreen ? popupScreen.height : 0`
- `barW` (real) — default `anchorWindow ? anchorWindow.width : 0`
- `barH` (real) — default `anchorWindow ? anchorWindow.height : 0`
- `availableCardWidth` (real) — default `screenW > 0`
- `availableCardHeight` (real) — default `screenH > 0`
- `verticalContentInset` (real) — default `padding * 2 + Border.top(borderSpec) + Border.bottom(bord...`

**Functions**
- `fittedContentWidth(width, cap)` (function)
- `fittedContentHeight(implicitHeight, cap)` (function)
- `cappedContentHeight(height)` (function)
- `close()` (function)

## ScreenMoveRemap

**Properties**
- `screen` (var) — default `window ? window.screen : null`
- `remapping` (bool) — default `false`

## SearchableDropdown

**Signals**
- `changed(string value)`
- `hovered(bool isHovered)`

**Properties**
- `label` (string) — default `""`
- `value` (string) — default `""`
- `options` (var) — default `[]`
- `placeholderText` (string) — default `"Search..."`
- `emptyText` (string) — default `"No matches"`
- `triggerLabel` (string) — default `""`
- `foreground` (color) — default `Color.popups.text`
- `background` (color) — default `Color.popups.background`
- `popupBorder` (color) — default `Color.popups.border`
- `accent` (color) — default `Color.accent`
- `popupBorderSpec` (var) — default `Border.localOrSurfaceSpec("popups", "border", popupBorder...`
- `fontFamily` (string) — default `Style.font.family`
- `rowHeight` (int) — default `Style.spacing.controlHeight`
- `popupRowHeight` (int) — default `Style.spacing.popupRowHeight`
- `popupMinHeight` (int) — default `Style.spacing.searchablePopupMinHeight`
- `showLabel` (bool) — default `true`
- `hasCursor` (bool) — default `false`
- `popupOpen` (bool) — default `popup.opened`
- `filtered` (var) — default `options`

**Functions**
- `open() { popup.open() }` (function)
- `close() { popup.close() }` (function)
- `toggle() { popup.opened ? popup.close() : popup.open() }` (function)
- `optionValue(o)` (function)
- `optionLabel(o)` (function)
- `optionDescription(o)` (function)
- `currentLabel()` (function)
- `recomputeFiltered()` (function)

## SpeedTestOverlay

**Signals**
- `closeRequested()`
- `runAgainRequested()`

**Properties**
- `unit` (string) — default `"Mbps"`
- `title` (string) — default `""`
- `layerNamespace` (string) — default `"omarchy-speed-test"`
- `runAgainTooltip` (string) — default `"Measure again"`
- `leftValue` (real) — default `0`
- `rightValue` (real) — default `0`
- `leftLive` (bool) — default `false`
- `rightLive` (bool) — default `false`
- `error` (string) — default `""`
- `open` (bool) — default `false`
- `scaleStops` (var) — default `[100, 250, 500, 1000, 2500, 5000, 10000]`
- `fullScale` (real) — default `scaleStops[0]`
- `failed` (bool) — default `error !== ""`
- `onScrim` (color) — default `"white"`
- `onScrimDim` (color) — default `Qt.rgba(1, 1, 1, 0.55)`
- `onScrimUrgent` (color) — default `"#ff6b6b"`

**Functions**
- `resetScale()` (function)
- `expandScale(value)` (function)

## TextField

**Properties**
- `foreground` (color) — default `Color.foreground`
- `accent` (color) — default `Color.accent`
- `selectionTint` (color) — default `Style.selectionFillFor(foreground, accent)`
- `password` (bool) — default `false`
- `horizontalPadding` (real) — default `Style.spacing.controlPaddingX`
- `verticalPadding` (real) — default `Style.spacing.inputPaddingY`
- `hasCursor` (bool) — default `false`
- `_focused` (bool) — default `activeFocus`
- `_hot` (bool) — default `hovered || hasCursor`
- `_borderSpec` (var) — default `Border.controlSpec(_focused ? "focus" : (_hot ? "hover-cu...`

## Toggle

**Signals**
- `clicked()`
- `hovered(bool isHovered)`

**Properties**
- `label` (string) — default `""`
- `description` (string) — default `""`
- `checked` (bool) — default `false`
- `hasCursor` (bool) — default `false`
- `rounded` (bool) — default `Style.cornerRadius > 0`
- `foreground` (color) — default `Color.foreground`
- `accent` (color) — default `Color.accent`
- `fontFamily` (string) — default `Style.font.family`
- `titleSize` (real) — default `Style.font.subtitle`
- `descriptionSize` (real) — default `Style.font.caption`
- `_hot` (bool) — default `hasCursor || mouse.containsMouse`
- `_borderSpec` (var) — default `Border.controlSpec(activeFocus ? "focus" : (_hot ? "hover...`

## ToggleSwitch

Bare on/off switch — no `label`/`description`/`clicked`. For a labeled
control use `Toggle` (label, description, checked, clicked).

**Signals**
- `toggled()`
- `hovered(bool isHovered)`

**Properties**
- `checked` (bool) — default `false`
- `busy` (bool) — default `false`
- `interactive` (bool) — default `true`
- `hasCursor` (bool) — default `false`
- `cursorRing` (bool) — default `interactive`
- `cursorPad` (int) — default `Style.space(6)`
- `rounded` (bool) — default `Style.cornerRadius > 0`
- `foreground` (color) — default `Color.foreground`
- `accent` (color) — default `Color.accent`
- `containsMouse` (alias) — default `mouse.containsMouse`
- `hot` (bool) — default `hasCursor || mouse.containsMouse`
- `trackHeight` (int) — default `Math.max(22, Math.round(Style.spacing.controlHeight * 0.55))`
- `trackWidth` (int) — default `Math.round(trackHeight * 1.9)`
- `knobSize` (int) — default `Math.max(6, Math.round(trackHeight * 0.72))`
- `knobInset` (int) — default `Math.max(1, Math.round((trackHeight - knobSize) / 2))`
- `_pad` (int) — default `cursorRing ? cursorPad : 0`

## WidgetButton

**Signals**
- `pressed(int button)`
- `wheelMoved(int delta)`

**Properties**
- `bar` (var) — default `null`
- `text` (string) — default `""`
- `fontFamily` (string) — default `bar ? bar.fontFamily : Style.font.family`
- `fontSize` (real) — default `Style.font.body`
- `foreground` (color) — default `bar ? bar.barForeground : Color.foreground`
- `activeColor` (color) — default `bar ? bar.urgent : Color.urgent`
- `active` (bool) — default `false`
- `horizontalMargin` (real) — default `8.5`
- `verticalPadding` (real) — default `6`
- `fixedWidth` (real) — default `-1`
- `fixedHeight` (real) — default `-1`
- `textRotation` (real) — default `0`
- `keepSpace` (bool) — default `false`
- `dimmed` (bool) — default `false`
- `concealed` (bool) — default `false`
- `interactive` (bool) — default `true`
- `pressable` (bool) — default `true`
- `useActiveColor` (bool) — default `true`
- `maintainIndicatorReveal` (bool) — default `false`
- `labelVisible` (bool) — default `true`
- `hasVisualContent` (bool) — default `text !== ""`
- `revealHost` (var) — default `bar`
- `tooltipText` (string) — default `""`
- `registeredBar` (var) — default `null`
- `vertical` (bool) — default `bar ? bar.vertical : false`
- `barSize` (int) — default `bar ? bar.barSize : Style.bar.sizeHorizontal`
- `scaledHorizontalMargin` (real) — default `Style.spaceReal(horizontalMargin)`
- `scaledVerticalPadding` (real) — default `Style.spaceReal(verticalPadding)`
- `tooltipHovered` (bool) — default `visible && interactive && !concealed && mouseArea.contain...`
- `labelWidth` (real) — default `label.visible ? label.implicitWidth : 0`

**Functions**
- `triggerPress(button)` (function)
- `hideOwnTooltip()` (function)
- `syncClickRegistration()` (function)
