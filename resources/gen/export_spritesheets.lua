-- Aseprite script: export one horizontal-strip PNG per (slice, tag) pair,
-- organised by resolution.
--
-- Naming convention:
--
--   Slices:
--     <role>_<res>            — 1-cell organism slice,  <res>x<res>
--     <role>_<res>_xl         — 2-cell organism slice,  <res>x(2*<res>)
--     <stem>_<res>_static     — static slice (no animation), paired only
--                                with the `base` tag. Used for food and any
--                                other single-frame art that doesn't cycle.
--
--   Tags:
--     <action>_<res>          — per-resolution action tag, spans (<res>/4)
--                                frames (1 at 4x4, 2 at 8x8, 4 at 16x16)
--     <action>_<res>_xl       — same, but for 2-cell (_xl) slices
--     base                    — single-frame tag; only pairs with _static
--                                slices. Lets one sprite file hold both
--                                animated and static art without the two
--                                categories cross-contaminating.
--
-- Pairing rules:
--
--   Static slice (ending _static):
--     pairs only with the `base` tag. Output filename strips the trailing
--     `_<res>_static` and the tag name entirely — e.g.
--       food_small_16_static × base  →  food_small.png
--
--   Non-static slice:
--     pairs with tags whose resolution AND _xl parity match. Output
--     filename strips `_<res>[_xl]` from both names — e.g.
--       small_16 × idle_16        →  small_idle.png
--       small_16_xl × move_16_xl  →  small_move.png
--       (small_16 × idle_8, small_16 × move_16_xl, etc. → skipped)
--
-- Output layout: files land under <outdir>/<res>x<res>/ — e.g.
-- <outdir>/16x16/small_idle.png. Missing subdirs are created.
--
-- Implementation: Aseprite's built-in ExportSpriteSheet handles blend
-- modes, palettes, and transparency exactly like the app. Since that
-- command has no "crop to slice" option, we save the sprite to a temp
-- file, open it as a scratch copy, and for each slice we crop the copy
-- to the slice bounds before running the export (then undo the crop
-- before moving on to the next slice).
--
-- Usage: File > Scripts > Run Script... in Aseprite and pick this file.
-- Point "Output folder" at your `resources/images/grid` directory and
-- this script will create / update the per-resolution subfolders inside.

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

local dlg = Dialog("Export (slice, tag) spritesheets")
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

-- Sanitise names for filesystem safety. Slice / tag names can contain
-- spaces, slashes, colons, etc.
local function safeName(s)
    return (s:gsub("[^%w%-_%.]", "_"))
end

-- endsWith reports whether s terminates with suffix.
local function endsWith(s, suffix)
    return #s >= #suffix and s:sub(-#suffix) == suffix
end

-- stripSuffix returns s with suffix removed from its tail. Caller must
-- have already verified endsWith(s, suffix) — this is just the slice.
local function stripSuffix(s, suffix)
    return s:sub(1, #s - #suffix)
end

-- parseSlice parses a slice name into its structural parts.
-- Returns nil if the name doesn't match the expected pattern — those
-- slices are skipped at export time.
--
-- Accepted shapes (checked in order):
--   <stem>_<res>_static   isStatic = true,  isXl = false
--   <stem>_<res>_xl       isStatic = false, isXl = true
--   <stem>_<res>          isStatic = false, isXl = false
local function parseSlice(name)
    local rest = name
    local isStatic = false
    local isXl = false

    if endsWith(rest, "_static") then
        isStatic = true
        rest = stripSuffix(rest, "_static")
    end
    if endsWith(rest, "_xl") then
        -- _xl + _static combinations aren't meaningful — treat them as
        -- malformed and skip. A single animation slice is either static
        -- (paired with `base`) or animated (paired with resolution tags).
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

-- parseTag parses a tag name into its structural parts. Returns nil for
-- unrecognised shapes (those are skipped).
--
-- Accepted shapes:
--   base                  isBase = true  (single-frame, for _static slices)
--   <action>_<res>_xl     isXl   = true
--   <action>_<res>        isXl   = false
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
    local action, resStr = rest:match("^(.-)_(%d+)$")
    if not action or not resStr or action == "" then
        return nil
    end
    return {
        action = action,
        res    = tonumber(resStr),
        isXl   = isXl,
        isBase = false,
    }
end

-- ensureDir creates the directory at `path` if it doesn't already exist.
-- app.fs.makeDirectory returns true on success (including when the
-- directory already existed).
local function ensureDir(path)
    if app.fs.isDirectory(path) then return true end
    return app.fs.makeDirectory(path)
end

-- Snapshot slice geometry and tag names before any cropping mutates the
-- scratch sprite. Shape-unparseable entries go in `skipped` so we can
-- report them at the end without polluting the main loop.
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
        table.insert(tags, info)
    end
end

-- Save a copy next to the source file and open it as the scratch sprite.
-- The temp filename is dotfile-prefixed so it sorts to the top and is
-- easy to recognise if something ever leaves it behind.
local tempPath = defaultDir .. sep .. ".tmp_export_spritesheets.aseprite"
spr:saveCopyAs(tempPath)

local origSprite = app.activeSprite
local copy = app.open(tempPath)
app.activeSprite = copy

-- Resolve indexed / grayscale palettes to true RGB on the scratch copy.
-- Without this, exporting an indexed-mode sprite as PNG can end up writing
-- the raw palette indices into the RGB channels (index 1 → RGB(1,1,1),
-- etc.), so the output looks like a black silhouette with correct alpha.
if copy.colorMode ~= ColorMode.RGB then
    app.command.ChangePixelFormat{ format = "rgb" }
end

local count = 0
for _, sl in ipairs(slices) do
    -- Set a selection covering the slice, then crop the scratch sprite to
    -- just that selection. Slice bounds reset to (0, 0) inside the cropped
    -- sprite, which is what ExportSpriteSheet then sees.
    copy.selection = Selection(Rectangle(sl.bounds.x, sl.bounds.y, sl.bounds.width, sl.bounds.height))
    app.command.CropSprite()

    local resDir = outDir .. sep .. tostring(sl.res) .. "x" .. tostring(sl.res)

    for _, t in ipairs(tags) do
        local pair, outName = false, nil
        if sl.isStatic then
            -- Static slice: only the single-frame `base` tag, output
            -- filename has no action suffix.
            if t.isBase then
                pair    = true
                outName = safeName(sl.stem) .. ".png"
            end
        else
            -- Animated slice: match on resolution AND xl parity, skip
            -- the base tag entirely.
            if (not t.isBase) and sl.res == t.res and sl.isXl == t.isXl then
                pair    = true
                outName = safeName(sl.stem) .. "_" .. safeName(t.action) .. ".png"
            end
        end

        if pair then
            if not ensureDir(resDir) then
                table.insert(skipped, "couldn't create " .. resDir)
            else
                local fn = resDir .. sep .. outName
                app.command.ExportSpriteSheet{
                    ui              = false,
                    type            = SpriteSheetType.HORIZONTAL,
                    textureFilename = fn,
                    tag             = t.name,
                    listLayers      = false,
                    listTags        = false,
                    listSlices      = false,
                }
                count = count + 1
                if dlg.data.verbose then
                    print("wrote " .. fn)
                end
            end
        end
    end

    -- Undo the crop so the next slice iterates against the full sprite.
    app.command.Undo()
end

-- Clean up: close the scratch copy without saving, remove the temp file,
-- restore focus to whatever sprite the user had active.
copy:close()
os.remove(tempPath)
if origSprite then
    app.activeSprite = origSprite
end

local msg = "Wrote " .. count .. " file(s) to\n" .. outDir
if #skipped > 0 then
    msg = msg .. "\n\nSkipped:\n- " .. table.concat(skipped, "\n- ")
end
app.alert(msg)
