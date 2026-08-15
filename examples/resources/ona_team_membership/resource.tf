resource "ona_team" "platform" {
  name = "Platform Engineering"
}

data "ona_user" "alice" {
  email          = "alice@example.com"
  login_provider = "github"
}

resource "ona_team_membership" "alice" {
  team_id = ona_team.platform.id
  user_id = data.ona_user.alice.user_id
}
