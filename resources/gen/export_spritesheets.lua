-- Aseprite script: export one horizontal-strip PNG per (slice, tag) pair.
--
-- For every named slice S and tag T in the active sprite, writes a file
-- named "<S>_<T>.png" to the chosen output directory. Each strip is one
-- tile wide per frame in the tag's range, cropped to the slice's bounds.
-- So a slice named "small" and a tag named "chemo" with 4 frames produces
-- a small_chemo.png that is (slice.width * 4) wide by slice.height tall.
--
-- Implementation: we use Aseprite's built-in ExportSpriteSheet command so
-- blend modes, palettes, transparency etc. render exactly like the app.
-- Since that command has no "crop to slice" option, we save the sprite to
-- a temp file, open it as a scratch copy, and for each slice we crop the
-- copy to the slice bounds before running the export (then undo the crop
-- before moving on to the next slice).
--
-- Usage: File > Scripts > Run Script... in Aseprite and pick this file.

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
    label = "Output folder",
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

-- Snapshot the slice geometry and tag names now, before cropping mutates
-- anything. Slices we can't handle (animated slices without static
-- bounds) are skipped with a note at the end.
local slices = {}
local skipped = {}
for _, slice in ipairs(spr.slices) do
    if slice.bounds then
        table.insert(slices, { name = slice.name, bounds = slice.bounds })
    else
        table.insert(skipped, "slice '" .. slice.name .. "' (no static bounds)")
    end
end
local tagNames = {}
for _, tag in ipairs(spr.tags) do
    table.insert(tagNames, tag.name)
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
-- Converting to RGB here forces Aseprite to look up each pixel's colour
-- through the active palette before the PNG is written. The original
-- source file is untouched — we mutate the scratch copy, which gets
-- closed without saving.
if copy.colorMode ~= ColorMode.RGB then
    app.command.ChangePixelFormat{ format = "rgb" }
end

local count = 0
for _, sl in ipairs(slices) do
    -- Set a selection covering the slice, then crop the scratch sprite
    -- to just that selection. Slice bounds reset to (0, 0) inside the
    -- cropped sprite, which is what ExportSpriteSheet sees.
    copy.selection = Selection(Rectangle(sl.bounds.x, sl.bounds.y, sl.bounds.width, sl.bounds.height))
    app.command.CropSprite()

    for _, tagName in ipairs(tagNames) do
        local fn = outDir .. sep .. safeName(sl.name) .. "_" .. safeName(tagName) .. ".png"
        app.command.ExportSpriteSheet{
            ui = false,
            type = SpriteSheetType.HORIZONTAL,
            textureFilename = fn,
            tag = tagName,
            listLayers = false,
            listTags = false,
            listSlices = false,
        }
        count = count + 1
        if dlg.data.verbose then
            print("wrote " .. fn)
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
