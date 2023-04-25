package main

import "strings"

var (
	Version   = ""
	Commit    = ""
	Origin    = ""
	BuildTime = ""
)

func fullVersion() string {
	info := []string{}
	if Commit != "" {
		info = append(info, "commit: "+Commit)
	}
	if BuildTime != "" {
		info = append(info, "build time: "+BuildTime)
	}
	if Origin != "" {
		info = append(info, "origin: "+Origin)
	}

	text := Version
	if text == "" {
		text = "0.0.0"
	}
	if s := strings.Join(info, ", "); s != "" {
		text += " (" + s + ")"
	}

	return text
}
