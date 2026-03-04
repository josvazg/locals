package cmds

import (
	"log"
	"strings"
)

func show(s *script, args ...string) error {
	log.Printf("Would run:\n%s %s\n\n"+
		"-------------------------------------------------------------\n"+
		"%s\n"+
		"-------------------------------------------------------------",
		s.name, strings.Join(args, " "), s.contents)
	return nil
}
