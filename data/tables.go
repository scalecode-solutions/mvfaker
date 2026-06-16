// Package data holds built-in generators and the registry of named field
// builders that config (HCL) and code share. Locale tables live here; the core
// gen package never imports this.
package data

var firstNames = []string{
	"James", "Mary", "Robert", "Patricia", "John", "Jennifer", "Michael", "Linda",
	"David", "Elizabeth", "William", "Barbara", "Richard", "Susan", "Joseph", "Jessica",
	"Thomas", "Sarah", "Aisha", "Wei", "Yuki", "Omar", "Priya", "Diego", "Nina", "Kofi",
}

var lastNames = []string{
	"Smith", "Johnson", "Williams", "Brown", "Jones", "Garcia", "Miller", "Davis",
	"Rodriguez", "Martinez", "Hernandez", "Lopez", "Gonzalez", "Wilson", "Anderson",
	"Nakamura", "Okafor", "Patel", "Khan", "Nguyen", "Silva", "Cohen", "Mwangi",
}

var domains = []string{
	"example.com", "mail.com", "inbox.dev", "post.io", "corp.net", "acme.org",
}

var words = []string{
	"lorem", "ipsum", "dolor", "sit", "amet", "consectetur", "adipiscing", "elit",
	"sed", "tempor", "labore", "magna", "aliqua", "enim", "minim", "veniam",
}

const hexDigits = "0123456789abcdef"
