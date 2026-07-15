resource "ona_workflow" "nightly_checks" {
  name        = "Nightly checks"
  description = "Runs repository checks every weekday."
  disabled    = false

  executor = {
    id        = "<service-account-id>"
    principal = "service_account"
  }

  triggers = [
    {
      manual = {}
      context = {
        projects = {
          project_ids = ["<project-id>"]
        }
      }
    },
    {
      time = {
        cron_expression = "0 9 * * 1-5"
      }
      context = {
        repositories = {
          repository_urls      = ["https://github.com/example-organization/example-repository"]
          environment_class_id = "<environment-class-id>"
        }
      }
    }
  ]

  action = {
    limits = {
      max_parallel = 2
      max_total    = 10
      max_time     = "30m"
    }

    steps = [
      {
        agent = {
          prompt = "Review the repository and identify failing checks."
        }
      },
      {
        task = {
          command = "make test"
        }
      }
    ]
  }
}
