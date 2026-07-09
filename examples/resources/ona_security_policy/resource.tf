resource "ona_security_policy" "baseline" {
  organization_id = "00000000-0000-0000-0000-000000000000"
  name            = "baseline"

  spec {
    ports {
      default_effect = "allow"

      rule {
        range_from = 22
        range_to   = 22
        effect     = "block"
      }
    }

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
