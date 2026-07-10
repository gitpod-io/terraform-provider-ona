resource "ona_security_policy" "port_controls" {
  organization_id = "00000000-0000-0000-0000-000000000000"
  name            = "port-controls"

  spec {
    ports {
      default_effect = "allow"

      rule {
        range_from = 22
        range_to   = 22
        effect     = "block"
      }
    }
  }
}

resource "ona_security_policy" "file_controls" {
  organization_id = "00000000-0000-0000-0000-000000000000"
  name            = "file-controls"

  spec {
    executables {
      default_effect = "allow"

      rule {
        path   = "/usr/bin/nc"
        effect = "audit"
      }
    }

    files {
      default_effect  = "allow"
      default_actions = ["read", "write"]

      rule {
        path    = "/etc/shadow"
        actions = ["read"]
        effect  = "block"
      }
    }
  }
}

resource "ona_security_policy" "data_controls" {
  organization_id = "00000000-0000-0000-0000-000000000000"
  name            = "data-controls"

  spec {
    block_devices {
      default_effect = "block"
    }

    data {
      default_effect = "allow"

      rule {
        source {
          file = "/workspace/secrets.env"
        }
        destination {
          host = "example.com"
        }
        effect = "block"
      }
    }
  }
}
