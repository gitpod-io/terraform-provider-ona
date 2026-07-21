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

      rule {
        path   = "curl"
        effect = "block"
      }
    }
  }
}

resource "ona_security_policy" "audit_tools" {
  organization_id = "00000000-0000-0000-0000-000000000000"
  name            = "audit-tools"

  spec {
    executables {
      default_effect = "allow"

      rule {
        path   = "/usr/bin/git"
        effect = "audit"
      }
    }
  }
}
