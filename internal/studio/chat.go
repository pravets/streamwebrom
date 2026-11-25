package studio

import "slices"

// stickerIDs are the built-in chat stickers; keep in sync with web/js/stickers.js.
var stickerIDs = []string{
	"hola", "bienvenidos", "gracias", "aplausos", "jaja", "fuego", "wow", "encanta",
	"gg", "like", "pregunta", "felicidades", "envivo", "saludos", "cafe", "yavuelvo",
}

func validSticker(id string) bool { return slices.Contains(stickerIDs, id) }

// DMPolicies are the allowed direct-message policies for State.DMPolicy.
// An empty policy (rooms saved before dm existed) means "all".
var DMPolicies = []string{"all", "host-only", "off"}

func dmPolicyOf(st State) string {
	if st.DMPolicy == "" {
		return "all"
	}
	return st.DMPolicy
}

// dmKey is the conversation key: the sorted pair of peer ids. Peer ids are
// base32 tokens, so "|" can never appear inside one.
func dmKey(a, b string) string {
	if a > b {
		a, b = b, a
	}
	return a + "|" + b
}
