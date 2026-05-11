-- Aseprite script: export one horizontal-strip PNG per (slice, tag,
-- layer) triple from a single source .aseprite holding all sprite
-- content at every resolution. Layers carve the composite into body
-- variants and feature overlays so the renderer can re-composite at
-- draw time; the script hides every other drawable layer when it
-- exports a target layer.
--
-- ============================================================
-- SLICE NAMING (existing convention, unchanged)
-- ============================================================
--   <stem>_<res>            — animated 1-cell slice,  <res>x<res>
--   <stem>_<res>_xl         — animated 2-cell slice,  <res>x(2*<res>)
--   <stem>_<res>_static     — static slice (food, wall, etc.);
--                              pairs only with the `base` tag.
--
--   Stems used by this project:
--     Organisms:  small, medium, large           (use animated slices)
--     Food:       food_small, food_medium,
--                 food_large                     (use _static slices)
--     Walls:      wall_weak, wall_medium,
--                 wall_strong                    (use _static slices;
--                                                 renderer picks the
--                                                 tier from current
--                                                 strength: 1-2 weak,
--                                                 3-5 medium, 6-7 strong)
--
-- ============================================================
-- TAG NAMING
-- ============================================================
--   <action>                — animated, ONE tag per action regardless
--                              of resolution. Author the tag with
--                              enough frames for the highest-res slice
--                              you intend to export — at 32x32 that's
--                              8 frames. The script slices off the
--                              right number of frames per export:
--                                4x4  → first 1 frame
--                                8x8  → first 2 frames
--                                16x16 → first 4 frames
--                                32x32 → first 8 frames
--                              A tag with fewer frames than the slice
--                              asks for is clamped, so a 4-frame tag
--                              still works for 32x32 (animation just
--                              plays the 4 frames in place of 8).
--   <action>_xl             — 2-cell variant, same frame-slicing rule.
--   base                    — single-frame, pairs only with _static slices.
--
-- ============================================================
-- LAYER NAMING (new — see applicableLayersFor for the
-- slice → layer mapping rules)
-- ============================================================
--
--   Drawable layers exported as their own PNG when the slice applies:
--
--     Organism low-res (4x4 / 8x8): only the unvaried body sprite.
--       body
--
--     Organism high-res (16x16 / 32x32): mutually-exclusive body
--     silhouettes + additive feature overlays. Every body variant
--     is exported for every organism size + action so the artist
--     deliberately considers each combination.
--       body_basic
--       body_shell
--       body_spikes
--       body_camouflage
--       flagellae
--       cilia
--       stinger
--       antennae
--       feelers
--       tasters
--       teeth
--       fangs
--       tusks
--
--     Static art (any resolution; paired only with the matching
--     _static slice stem):
--       food
--       wall
--
--   Preview-only layers — never exported, even when visible:
--     background            — backdrop you can use to see how
--                              sprites read in-context while drawing.
--     _<anything>           — layers whose name starts with `_` are
--                              treated as drafts / references and skipped.
--
-- ============================================================
-- OUTPUT FILENAMES
-- ============================================================
--
--   Slices with one applicable layer (low-res organism, food, wall):
--     animated:   <stem>_<action>.png       e.g. small_idle.png
--     static:     <stem>.png                e.g. food_small.png, wall.png
--
--   Slices with multiple applicable layers (high-res organism):
--     animated:   <layer>_<stem>_<action>.png
--                                           e.g. body_basic_small_idle.png
--                                                flagellae_small_move.png
--                                                teeth_medium_eat.png
--
--   Output dir: <outdir>/<res>x<res>/<filename>
--
-- ============================================================
-- IMPLEMENTATION
-- ============================================================
--
-- Aseprite's ExportSpriteSheet honours the current layer visibility
-- on the active sprite. To isolate one layer per export we save the
-- sprite to a scratch copy, then for each (slice, layer, tag) we:
--   1. Crop the scratch sprite to the slice bounds (so the export
--      stamps the right cell region).
--   2. Hide every drawable layer except the current target. Preview
--      layers (background, _-prefixed) stay hidden the whole time.
--   3. Run ExportSpriteSheet for the matching tag.
--   4. Undo the crop and move on.
-- The scratch copy is discarded at the end, so the user's sprite
-- and layer visibility are untouched by the script.
--
-- Usage: File > Scripts > Run Script... in Aseprite and pick this
-- file. Point "Output folder" at your `resources/images/grid`
-- directory and the script creates / updates the per-resolution
-- subfolders inside.

local spr = app.activeSprite
if not spr then
    return app.alert("No active sprite.")
end
if not spr.filename or spr.filename == "" then
    return app.alert("Save the sprite to a file first — the script makes a scratch copy on disk.")
end
if #spr.slices == 0 then
    return app.alert("Sprite has no slices defined.")
end
if #spr.tags == 0 then
    return app.alert("Sprite has no tags defined.")
end

local sep = app.fs.pathSeparator or "/"
local defaultDir = app.fs.filePath(spr.filename)

local dlg = Dialog("Export (slice, tag, layer) spritesheets")
dlg:entry{
    id = "dir",
    label = "Output folder (grid dir)",
    text = defaultDir,
}
dlg:check{
    id = "verbose",
    label = "Print each file path",
    selected = true,
}
dlg:button{ id = "ok", text = "Export", focus = true }
dlg:button{ id = "cancel", text = "Cancel" }
dlg:show()
if not dlg.data.ok then return end

local outDir = dlg.data.dir
if outDir == nil or outDir == "" then outDir = defaultDir end

-- ------------------------------------------------------------------
-- Helpers
-- ------------------------------------------------------------------

-- Sanitise names for filesystem safety. Slice / tag / layer names
-- can contain spaces, slashes, colons, etc.
local function safeName(s)
    return (s:gsub("[^%w%-_%.]", "_"))
end

local function endsWith(s, suffix)
    return #s >= #suffix and s:sub(-#suffix) == suffix
end

local function startsWith(s, prefix)
    return #s >= #prefix and s:sub(1, #prefix) == prefix
end

local function stripSuffix(s, suffix)
    return s:sub(1, #s - #suffix)
end

-- isPreviewLayer reports whether a layer should never appear in any
-- exported PNG (including as part of the composite that
-- ExportSpriteSheet draws). Reserved names are `background` and
-- anything starting with `_` or `.` — useful for guide / draft /
-- reference layers the artist wants only inside Aseprite.
local function isPreviewLayer(name)
    if name == "background" then return true end
    local first = name:sub(1, 1)
    if first == "_" or first == "." then return true end
    return false
end

-- parseSlice mirrors the existing convention.
local function parseSlice(name)
    local rest = name
    local isStatic = false
    local isXl = false

    if endsWith(rest, "_static") then
        isStatic = true
        rest = stripSuffix(rest, "_static")
    end
    if endsWith(rest, "_xl") then
        if isStatic then return nil end
        isXl = true
        rest = stripSuffix(rest, "_xl")
    end

    local stem, resStr = rest:match("^(.-)_(%d+)$")
    if not stem or not resStr or stem == "" then
        return nil
    end
    return {
        stem     = stem,
        res      = tonumber(resStr),
        isXl     = isXl,
        isStatic = isStatic,
    }
end

-- parseTag accepts:
--   "base"          — static, paired only with _static slices.
--   "<action>"      — animated, 1-cell.
--   "<action>_xl"   — animated, 2-cell.
-- Returns nil for any other shape so unrecognised tags get skipped
-- and reported.
local function parseTag(name)
    if name == "base" then
        return { isBase = true }
    end
    local rest = name
    local isXl = false
    if endsWith(rest, "_xl") then
        isXl = true
        rest = stripSuffix(rest, "_xl")
    end
    if rest == "" then
        return nil
    end
    return {
        action = rest,
        isXl   = isXl,
        isBase = false,
    }
end

-- frameNumberOf normalises Aseprite's two API shapes for Tag.fromFrame /
-- Tag.toFrame — older versions return a 1-indexed int, recent
-- versions return a Frame object whose frameNumber is the index.
local function frameNumberOf(f)
    if type(f) == "number" then
        return f
    end
    if f and f.frameNumber then
        return f.frameNumber
    end
    return 1
end

-- applicableLayersFor returns the ordered list of layer names that
-- should produce output for the given slice. Returns nil to mean
-- "no exports for this slice" (skipped). The mapping is:
--   wall*           → { "wall" }
--   food_*          → { "food" }
--   organism @ <=8  → { "body" }
--   organism @ >=16 → { body variants ..., feature overlays ... }
local function applicableLayersFor(slice)
    local stem = slice.stem
    if startsWith(stem, "wall") then
        return { "wall" }
    end
    if startsWith(stem, "food_") or stem == "food" then
        return { "food" }
    end
    -- Anything else is treated as an organism slice. Resolution
    -- decides whether to enumerate body variants + overlays or just
    -- use the single low-res body.
    if slice.res <= 8 then
        return { "body" }
    end
    return {
        "body_basic", "body_shell", "body_spikes", "body_camouflage",
        "flagellae", "cilia", "stinger",
        "antennae", "feelers", "tasters",
        "teeth", "fangs", "tusks",
    }
end

local function ensureDir(path)
    if app.fs.isDirectory(path) then return true end
    return app.fs.makeDirectory(path)
end

-- ------------------------------------------------------------------
-- Parse slices + tags from the source sprite (geometry is needed
-- before we crop the scratch copy).
-- ------------------------------------------------------------------

local slices = {}
local skipped = {}
for _, slice in ipairs(spr.slices) do
    if not slice.bounds then
        table.insert(skipped, "slice '" .. slice.name .. "' (no static bounds)")
    else
        local info = parseSlice(slice.name)
        if not info then
            table.insert(skipped, "slice '" .. slice.name .. "' (unrecognised name)")
        else
            info.name   = slice.name
            info.bounds = slice.bounds
            table.insert(slices, info)
        end
    end
end

local tags = {}
for _, tag in ipairs(spr.tags) do
    local info = parseTag(tag.name)
    if not info then
        table.insert(skipped, "tag '" .. tag.name .. "' (unrecognised name)")
    else
        info.name = tag.name
        info.fromFrame = frameNumberOf(tag.fromFrame)
        info.toFrame   = frameNumberOf(tag.toFrame)
        table.insert(tags, info)
    end
end

-- ------------------------------------------------------------------
-- Sanity check: warn about expected slices / tags / layers that
-- aren't in the source file (catches typos and forgotten content),
-- and any drawable layers whose names don't match the project's
-- known set (catches misspelled layer names). The script still
-- runs and exports whatever it can — the warnings are surfaced in
-- the final summary so the user can fix and re-export.
-- ------------------------------------------------------------------

-- Lookup tables built from what's actually present.
local presentSliceNames = {}
local presentTagNames = {}
local presentLayerNames = {}
local resolutionsPresent = {}
local hasLowResOrganism = false
local hasHighResOrganism = false
local hasAnyXlSlice = false
local hasAnyAnimatedNonXlSlice = false
local hasAnyStaticSlice = false
local organismStemSet = { small = true, medium = true, large = true }

for _, sl in ipairs(slices) do
    presentSliceNames[sl.name] = true
    resolutionsPresent[sl.res] = true
    if organismStemSet[sl.stem] then
        if sl.res <= 8 then hasLowResOrganism = true end
        if sl.res >= 16 then hasHighResOrganism = true end
    end
    if sl.isXl then hasAnyXlSlice = true end
    if sl.isStatic then
        hasAnyStaticSlice = true
    elseif not sl.isXl then
        hasAnyAnimatedNonXlSlice = true
    end
end
for _, t in ipairs(tags) do
    presentTagNames[t.name] = true
end
for _, layer in ipairs(spr.layers) do
    presentLayerNames[layer.name] = true
end

-- Expected sets.
local expectedTagActions = {
    "idle", "move", "blocked", "turn_left", "turn_right",
    "attack", "eat", "eatfail", "chemo", "chemofail", "die",
}
local lowResOrganismLayers  = { "body" }
local highResOrganismLayers = {
    "body_basic", "body_shell", "body_spikes", "body_camouflage",
    "flagellae", "cilia", "stinger",
    "antennae", "feelers", "tasters",
    "teeth", "fangs", "tusks",
}
local universalLayers   = { "food", "wall" }
local expectedOrgStems  = { "small", "medium", "large" }
local expectedFoodStems = { "food_small", "food_medium", "food_large" }
local expectedWallStems = { "wall_weak", "wall_medium", "wall_strong" }

-- Build the full known-layer set so we can flag typos
-- (drawable layers whose name isn't in the project's vocabulary).
local knownLayerNames = {}
for _, n in ipairs(lowResOrganismLayers) do knownLayerNames[n] = true end
for _, n in ipairs(highResOrganismLayers) do knownLayerNames[n] = true end
for _, n in ipairs(universalLayers) do knownLayerNames[n] = true end

local missing = {}
local unknown = {}

-- Tags: each expected action is wired by exactly ONE tag variant —
-- either the bare `<action>` (uses the regular slice) or
-- `<action>_xl` (uses the wider 2-cell slice). The suffix is just
-- an indicator of which slice geometry the action drives, not a
-- demand for two parallel animations. So we only warn when neither
-- variant exists. `base` is still required separately by static slices.
local hasAnyAnimatedSlice = hasAnyAnimatedNonXlSlice or hasAnyXlSlice
if hasAnyAnimatedSlice then
    for _, action in ipairs(expectedTagActions) do
        if not presentTagNames[action] and not presentTagNames[action .. "_xl"] then
            table.insert(missing, "tag '" .. action .. "' or '" .. action .. "_xl'")
        end
    end
end
if hasAnyStaticSlice and not presentTagNames["base"] then
    table.insert(missing, "tag 'base' (required by static slices)")
end

-- Layers: universal layers + body-tier layers driven by which
-- organism resolutions are in use.
local expectedLayers = {}
for _, n in ipairs(universalLayers) do
    table.insert(expectedLayers, n)
end
if hasLowResOrganism then
    for _, n in ipairs(lowResOrganismLayers) do
        table.insert(expectedLayers, n)
    end
end
if hasHighResOrganism then
    for _, n in ipairs(highResOrganismLayers) do
        table.insert(expectedLayers, n)
    end
end
for _, n in ipairs(expectedLayers) do
    if not presentLayerNames[n] then
        table.insert(missing, "layer '" .. n .. "'")
    end
end

-- Unrecognised drawable layers — anything not in the known set,
-- not a preview layer, and not a draft prefix. Likely typos.
for _, layer in ipairs(spr.layers) do
    if not isPreviewLayer(layer.name) and not knownLayerNames[layer.name] then
        table.insert(unknown, "layer '" .. layer.name .. "' (not in the project's known layer set — typo?)")
    end
end

-- Slices: for each resolution the user has touched, check that
-- the canonical stem set is present. Resolutions the user hasn't
-- authored at all are silently allowed (work-in-progress).
local function checkSlice(name)
    if not presentSliceNames[name] then
        table.insert(missing, "slice '" .. name .. "'")
    end
end
for res in pairs(resolutionsPresent) do
    local s = tostring(res)
    for _, stem in ipairs(expectedOrgStems) do
        checkSlice(stem .. "_" .. s)
    end
    for _, stem in ipairs(expectedFoodStems) do
        checkSlice(stem .. "_" .. s .. "_static")
    end
    for _, stem in ipairs(expectedWallStems) do
        checkSlice(stem .. "_" .. s .. "_static")
    end
end

-- ------------------------------------------------------------------
-- Open a scratch copy of the sprite. We crop and toggle layer
-- visibility on the copy so the user's source sprite is untouched.
-- ------------------------------------------------------------------

local tempPath = defaultDir .. sep .. ".tmp_export_spritesheets.aseprite"
spr:saveCopyAs(tempPath)

local origSprite = app.activeSprite
local copy = app.open(tempPath)
app.activeSprite = copy

if copy.colorMode ~= ColorMode.RGB then
    app.command.ChangePixelFormat{ format = "rgb" }
end

-- Build a name → layer lookup on the scratch copy so per-layer
-- visibility toggles are O(1) instead of O(N) per iteration.
local layersByName = {}
for _, layer in ipairs(copy.layers) do
    layersByName[layer.name] = layer
end

-- setSoloLayer hides every drawable layer except the target. Preview
-- layers stay hidden for the entire export regardless of their
-- starting visibility.
local function setSoloLayer(targetName)
    for _, layer in ipairs(copy.layers) do
        if isPreviewLayer(layer.name) then
            layer.isVisible = false
        else
            layer.isVisible = (layer.name == targetName)
        end
    end
end

local function outputFilename(slice, tag, layerName, applicable)
    local stem = safeName(slice.stem)
    if #applicable == 1 then
        if slice.isStatic then
            return stem .. ".png"
        end
        return stem .. "_" .. safeName(tag.action) .. ".png"
    end
    return safeName(layerName) .. "_" .. stem .. "_" .. safeName(tag.action) .. ".png"
end

-- ------------------------------------------------------------------
-- Main export loop: (slice × applicable layer × matching tag).
-- ------------------------------------------------------------------

local count = 0
for _, sl in ipairs(slices) do
    copy.selection = Selection(Rectangle(sl.bounds.x, sl.bounds.y, sl.bounds.width, sl.bounds.height))
    app.command.CropSprite()

    local resDir = outDir .. sep .. tostring(sl.res) .. "x" .. tostring(sl.res)
    local applicable = applicableLayersFor(sl)

    if applicable then
        for _, layerName in ipairs(applicable) do
            local target = layersByName[layerName]
            if target == nil then
                table.insert(skipped, "layer '" .. layerName .. "' applies to slice '" .. sl.name .. "' but is missing from the sprite")
            else
                setSoloLayer(layerName)

                for _, t in ipairs(tags) do
                    local pair, outName = false, nil
                    if sl.isStatic then
                        if t.isBase then
                            pair    = true
                            outName = outputFilename(sl, t, layerName, applicable)
                        end
                    else
                        -- Animated slice + animated tag: pair on
                        -- xl parity only. Resolution no longer needs
                        -- to match because one tag serves every res
                        -- via the frame-slicing rule below.
                        if (not t.isBase) and sl.isXl == t.isXl then
                            pair    = true
                            outName = outputFilename(sl, t, layerName, applicable)
                        end
                    end

                    if pair then
                        if not ensureDir(resDir) then
                            table.insert(skipped, "couldn't create " .. resDir)
                        else
                            local fn = resDir .. sep .. outName
                            -- Frame range: each resolution takes the
                            -- first (res/4) frames from the tag's
                            -- range. Static (base) tags have
                            -- fromFrame == toFrame so the math
                            -- collapses to a single frame.
                            local framesNeeded = sl.res / 4
                            if framesNeeded < 1 then
                                framesNeeded = 1
                            end
                            local exportFrom = t.fromFrame
                            local exportTo   = t.fromFrame + framesNeeded - 1
                            if t.isBase then
                                exportTo = t.toFrame
                            elseif exportTo > t.toFrame then
                                -- Tag has fewer frames than the
                                -- slice asked for — clamp and let
                                -- the animation play in place
                                -- rather than walking off the end.
                                exportTo = t.toFrame
                            end
                            -- Build the horizontal strip manually.
                            -- app.command.ExportSpriteSheet has no
                            -- frame-range params (only `tag` filters
                            -- frames, and only at tag granularity),
                            -- so we stamp each wanted frame onto a
                            -- blank image and save that directly.
                            -- Image:drawSprite honours the current
                            -- layer visibility set by setSoloLayer.
                            local cellW = sl.bounds.width
                            local cellH = sl.bounds.height
                            local frameCount = exportTo - exportFrom + 1
                            local strip = Image(cellW * frameCount, cellH, copy.colorMode)
                            for i = 0, frameCount - 1 do
                                strip:drawSprite(copy, exportFrom + i, Point(i * cellW, 0))
                            end
                            strip:saveAs(fn)
                            count = count + 1
                            if dlg.data.verbose then
                                print("wrote " .. fn)
                            end
                        end
                    end
                end
            end
        end
    end

    app.command.Undo()
end

-- ------------------------------------------------------------------
-- Tear down the scratch copy. The user's source sprite was never
-- touched, so there's no visibility state to restore there.
-- ------------------------------------------------------------------

copy:close()
os.remove(tempPath)
if origSprite then
    app.activeSprite = origSprite
end

-- app.alert renders `\n` inside a single string as literal characters,
-- so build a table of lines and pass it as the `text` field — each
-- entry becomes its own line in the dialog.
local lines = { "Wrote " .. count .. " file(s) to", outDir }
local function appendSection(header, items)
    if #items == 0 then return end
    table.insert(lines, "")
    table.insert(lines, header)
    for _, item in ipairs(items) do
        table.insert(lines, "- " .. item)
    end
end
appendSection("Missing (expected but not found):", missing)
appendSection("Unrecognised:", unknown)
appendSection("Skipped:", skipped)
app.alert{ title = "Export spritesheets", text = lines }
