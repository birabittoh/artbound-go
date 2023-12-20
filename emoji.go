package main

type EmojiDict struct {
	Color      string
	ColorBlack string
	Favicon    string
	Get        string
	GetFirst   string
	Help       string
	Home       string
	Next       string
	Prev       string
	Rotate     string
	SelectAll  string
	SelectNone string
	Save       string
	SaveIG     string
	Toggle     string
}

var defaultEmojis = EmojiDict{
	"⚪",
	"⚫",
	"✏️",
	"⏬",
	"🔽",
	"❔",
	"🏠",
	"➡️",
	"⬅️",
	"🔁",
	"✅",
	"❎",
	"💾",
	"📷",
	"♻️",
}
