package ui

// iconSet holds the glyphs drawn beside labels. Each includes its trailing
// space, so an empty icon takes no room at all.
type iconSet struct {
	folder, folderOpen string
	requests, history  string
	environment, host  string
	send               string
	info, success      string
	warning, failure   string
	elapsed            string
}

// plainIcons work in any font.
var plainIcons = iconSet{
	environment: "◆ ",
}

// nerdIcons need a Nerd Font (https://www.nerdfonts.com): Font Awesome
// glyphs from its private use area.
var nerdIcons = iconSet{
	folder:      " ",
	folderOpen:  " ",
	requests:    " ",
	history:     " ",
	environment: " ",
	host:        " ",
	send:        " ",
	info:        " ",
	success:     " ",
	warning:     " ",
	failure:     " ",
	elapsed:     " ",
}

func iconsFor(nerdFonts bool) iconSet {
	if nerdFonts {
		return nerdIcons
	}
	return plainIcons
}
