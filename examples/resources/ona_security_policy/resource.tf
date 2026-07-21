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

resource "ona_security_policy" "default_allow" {
  organization_id = "00000000-0000-0000-0000-000000000000"
  name            = "default-allow"

  spec {
    executables {
      default_effect = "allow"
    }
  }
}

resource "ona_security_policy" "default_block" {
  organization_id = "00000000-0000-0000-0000-000000000000"
  name            = "default-block"

  spec {
    executables {
      default_effect = "block"
    }
  }
}
