// Package data holds built-in generators and the registry of named field
// builders that config (HCL) and code share. Locale tables live here; the core
// gen package never imports this.
package data

// firstNames and lastNames live in dataset_gen.go (US Census / SSA-derived).

var domains = []string{
	"example.com", "mail.com", "inbox.dev", "post.io", "corp.net", "acme.org",
}

var words = []string{
	"lorem", "ipsum", "dolor", "sit", "amet", "consectetur", "adipiscing", "elit",
	"sed", "tempor", "labore", "magna", "aliqua", "enim", "minim", "veniam",
}

const hexDigits = "0123456789abcdef"
