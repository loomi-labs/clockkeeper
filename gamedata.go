package clockkeeper

import (
	"embed"
)

//go:embed data/roles.json
var RolesJSON []byte

//go:embed data/nightsheet.json
var NightSheetJSON []byte

//go:embed data/jinxes.json
var JinxesJSON []byte

//go:embed all:data/characters
var CharacterIcons embed.FS
