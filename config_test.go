package mitm

import (
  "log"
  "testing"
)

func TestParseCommandlineOptions(t *testing.T) {
  args := []string{"-r", "client", "-c", "config.yml"}
  options, err := ParseCommandlineOptions(args)
  if err != nil {
    log.Println("error in parsing options:", err.Error())
    return
  }
  if options.Role != "client" || options.ConfigPath != "config.yml" {
    t.Errorf("Role = <%s>, want = <%s>; ConfigPath = <%s>, want = <%s>",
      options.Role, "client", options.ConfigPath, "config.yml")
  }
}
