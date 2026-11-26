package studio

import (
	"slices"
	"strings"
)

// stickerIDs are the built-in chat stickers; keep in sync with web/js/stickers.js.
var stickerIDs = []string{
	"hola", "bienvenidos", "gracias", "aplausos", "jaja", "fuego", "wow", "encanta",
	"gg", "like", "pregunta", "felicidades", "envivo", "saludos", "cafe", "yavuelvo",
}

func validSticker(id string) bool { return slices.Contains(stickerIDs, id) }

// dmPolicies are the allowed direct-message policies for State.DMPolicy.
var dmPolicies = []string{"all", "host-only", "off"}

// validDMPolicy reports whether a policy string may be stored in the room.
func validDMPolicy(policy string) bool { return slices.Contains(dmPolicies, policy) }

// dmPolicyOf normalises a stored policy. An empty policy (rooms saved before
// dm existed) means "all"; anything unknown fails closed, so a typo can never
// switch direct messages on behind the host's back.
func dmPolicyOf(st State) string {
	switch st.DMPolicy {
	case "", "all":
		return "all"
	case "host-only":
		return "host-only"
	case "off":
		return "off"
	default:
		return "off"
	}
}

// dmKey is the conversation key: the sorted pair of peer ids. Peer ids are
// base32 tokens, so "|" can never appear inside one.
func dmKey(a, b string) string {
	if a > b {
		a, b = b, a
	}
	return a + "|" + b
}

// dmKeyParts splits a conversation key back into its peer ids.
func dmKeyParts(key string) (a, b string, ok bool) { return strings.Cut(key, "|") }
