resource "ona_security_policy" "baseline" {
  organization_id = "00000000-0000-0000-0000-000000000000"
  name            = "baseline"

  spec {
    executables {
      default_effect = "allow"

      rule {
        path   = "/usr/bin/nc"
        effect = "audit"
      }
    }
  }
}

resource "ona_security_policy" "executables_only" {
  organization_id = "00000000-0000-0000-0000-000000000000"
  name            = "executable-controls"

  spec {
    executables {
      default_effect = "block"

      rule {
        path   = "/usr/bin/git"
        effect = "allow"
      }
    }
  }
}
