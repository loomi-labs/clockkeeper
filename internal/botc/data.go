// Package botc provides Blood on the Clocktower game data and rules.
package botc

// Team represents a character's team affiliation.
type Team string

const (
	TeamTownsfolk Team = "townsfolk"
	TeamOutsider  Team = "outsider"
	TeamMinion    Team = "minion"
	TeamDemon     Team = "demon"
	TeamTraveller Team = "traveller"
	TeamFabled    Team = "fabled"
)

// Character represents a Blood on the Clocktower character loaded from roles.json.
type Character struct {
	ID                 string   `json:"id"`
	Name               string   `json:"name"`
	Team               Team     `json:"team"`
	Edition            string   `json:"edition"`
	Ability            string   `json:"ability"`
	Flavor             string   `json:"flavor"`
	Setup              bool     `json:"setup"`
	Reminders          []string `json:"reminders"`
	RemindersGlobal    []string `json:"remindersGlobal"`
	FirstNight         int      `json:"firstNight"`
	FirstNightReminder string   `json:"firstNightReminder"`
	OtherNight         int      `json:"otherNight"`
	OtherNightReminder string   `json:"otherNightReminder"`
}

// Jinx represents an interaction rule between two characters.
type Jinx struct {
	ID     string `json:"id"`
	Reason string `json:"reason"`
}

// CharacterJinxes groups all jinxes for a single character.
type CharacterJinxes struct {
	ID    string `json:"id"`
	Jinx  []Jinx `json:"jinx"`
}

// NightSheet defines the wake order for first and other nights.
type NightSheet struct {
	FirstNight []string `json:"firstNight"`
	OtherNight []string `json:"otherNight"`
}

// Edition represents a built-in character set (e.g., Trouble Brewing).
type Edition struct {
	ID         string
	Name       string
	Characters []*Character
}

// KnownEditions lists the official edition IDs and display names.
var KnownEditions = []struct {
	ID   string
	Name string
}{
	{"tb", "Trouble Brewing"},
	{"bmr", "Bad Moon Rising"},
	{"snv", "Sects & Violets"},
}
