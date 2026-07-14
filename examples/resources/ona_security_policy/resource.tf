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

      rule {
        source {
          integration = "00000000-0000-0000-0000-000000000001"
          selector    = "example-organization/private-repository"
        }
        destination {
          host = "api.example.com"
        }
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
      default_effect = "allow"

      rule {
        range_from = 22
        range_to   = 22
        effect     = "block"
      }
    }

    executables {
      default_effect = "allow"
    }

    files {
      default_effect = "allow"
    }

    block_devices {
      default_effect = "allow"
    }

    data {
      default_effect = "allow"
    }
  }
}

resource "ona_security_policy" "files_only" {
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

    ports {
      default_effect = "allow"
    }

    block_devices {
      default_effect = "allow"
    }

    data {
      default_effect = "allow"
    }
  }
}

resource "ona_security_policy" "data_only" {
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

      rule {
        source {
          integration = "00000000-0000-0000-0000-000000000001"
          selector    = "example-organization/private-repository"
        }
        destination {
          host = "api.example.com"
        }
        effect = "audit"
      }
    }

    ports {
      default_effect = "allow"
    }

    executables {
      default_effect = "allow"
    }

    files {
      default_effect = "allow"
    }
  }
}
