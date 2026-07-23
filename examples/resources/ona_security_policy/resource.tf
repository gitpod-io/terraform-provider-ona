resource "ona_security_policy" "baseline" {
  organization_id = "00000000-0000-0000-0000-000000000000"
  name            = "baseline"

  spec {
    ports {
      max_admission_level = "organization"
    }

    executables {
      default_effect = "allow"

      rule {
        path   = "/usr/bin/nc"
        effect = "audit"
      }
    }
  }
}

resource "ona_security_policy" "ports_only" {
  organization_id = "00000000-0000-0000-0000-000000000000"
  name            = "port-controls"

  spec {
    ports {
      max_admission_level = "creator_only"
    }
  }
}

resource "ona_security_policy" "executables_only" {
  organization_id = "00000000-0000-0000-0000-000000000000"
  name            = "executable-controls"

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
