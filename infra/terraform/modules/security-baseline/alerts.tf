# Alerts by email: spending, GuardDuty findings, root account sign-in.

resource "aws_budgets_budget" "monthly" {
  name         = "${var.name}-monthly"
  budget_type  = "COST"
  limit_amount = tostring(var.budget_limit_usd)
  limit_unit   = "USD"
  time_unit    = "MONTHLY"

  notification {
    comparison_operator        = "GREATER_THAN"
    threshold                  = 50
    threshold_type             = "PERCENTAGE"
    notification_type          = "ACTUAL"
    subscriber_email_addresses = [var.alert_email]
  }

  notification {
    comparison_operator        = "GREATER_THAN"
    threshold                  = 100
    threshold_type             = "PERCENTAGE"
    notification_type          = "ACTUAL"
    subscriber_email_addresses = [var.alert_email]
  }

  notification {
    comparison_operator        = "GREATER_THAN"
    threshold                  = 100
    threshold_type             = "PERCENTAGE"
    notification_type          = "FORECASTED"
    subscriber_email_addresses = [var.alert_email]
  }
}

resource "aws_ce_anomaly_monitor" "services" {
  count             = var.create_cost_anomaly_monitor ? 1 : 0
  name              = "${var.name}-services"
  monitor_type      = "DIMENSIONAL"
  monitor_dimension = "SERVICE"
}

resource "aws_ce_anomaly_subscription" "email" {
  count            = var.create_cost_anomaly_monitor ? 1 : 0
  name             = "${var.name}-anomalies"
  frequency        = "DAILY"
  monitor_arn_list = [aws_ce_anomaly_monitor.services[0].arn]

  subscriber {
    type    = "EMAIL"
    address = var.alert_email
  }

  threshold_expression {
    dimension {
      key           = "ANOMALY_TOTAL_IMPACT_ABSOLUTE"
      match_options = ["GREATER_THAN_OR_EQUAL"]
      values        = ["2"]
    }
  }
}

resource "aws_guardduty_detector" "this" {
  enable                       = true
  finding_publishing_frequency = "FIFTEEN_MINUTES"
}

resource "aws_sns_topic" "alerts" {
  name              = "${var.name}-security-alerts"
  kms_master_key_id = aws_kms_key.main.id
}

resource "aws_sns_topic_subscription" "alerts_email" {
  topic_arn = aws_sns_topic.alerts.arn
  protocol  = "email"
  endpoint  = var.alert_email
}

resource "aws_cloudwatch_event_rule" "guardduty" {
  name        = "${var.name}-guardduty-findings"
  description = "GuardDuty findings of medium severity or higher"
  event_pattern = jsonencode({
    source        = ["aws.guardduty"]
    "detail-type" = ["GuardDuty Finding"]
    detail = {
      severity = [{ numeric = [">=", 4] }]
    }
  })
}

resource "aws_cloudwatch_event_rule" "root_login" {
  name        = "${var.name}-root-login"
  description = "Any console sign-in with the root user"
  event_pattern = jsonencode({
    "detail-type" = ["AWS Console Sign In via CloudTrail"]
    detail = {
      userIdentity = { type = ["Root"] }
    }
  })
}

# Root console sign-ins may be recorded in us-east-1 whatever the home region, so forward them to the
# home region's default bus, where the rule above sends them to SNS.
resource "aws_cloudwatch_event_rule" "root_login_forward" {
  count         = local.forward_global_events ? 1 : 0
  provider      = aws.us_east_1
  name          = "${var.name}-root-login-forward"
  description   = "Forwards root sign-ins recorded in us-east-1 to ${local.region}"
  event_pattern = aws_cloudwatch_event_rule.root_login.event_pattern
}

resource "aws_cloudwatch_event_target" "root_login_forward" {
  count    = local.forward_global_events ? 1 : 0
  provider = aws.us_east_1
  rule     = aws_cloudwatch_event_rule.root_login_forward[0].name
  arn      = local.home_bus_arn
  role_arn = aws_iam_role.events_forward[0].arn
}

data "aws_iam_policy_document" "events_forward_trust" {
  statement {
    actions = ["sts:AssumeRole"]
    principals {
      type        = "Service"
      identifiers = ["events.amazonaws.com"]
    }
    condition {
      test     = "StringEquals"
      variable = "aws:SourceAccount"
      values   = [local.account_id]
    }
  }
}

data "aws_iam_policy_document" "events_forward" {
  statement {
    actions   = ["events:PutEvents"]
    resources = [local.home_bus_arn]
  }
}

resource "aws_iam_role" "events_forward" {
  count              = local.forward_global_events ? 1 : 0
  name               = "${var.name}-events-forward"
  assume_role_policy = data.aws_iam_policy_document.events_forward_trust.json
}

resource "aws_iam_role_policy" "events_forward" {
  count  = local.forward_global_events ? 1 : 0
  name   = "put-home-bus"
  role   = aws_iam_role.events_forward[0].id
  policy = data.aws_iam_policy_document.events_forward.json
}

resource "aws_cloudwatch_event_target" "alerts" {
  for_each = {
    guardduty  = aws_cloudwatch_event_rule.guardduty.name
    root_login = aws_cloudwatch_event_rule.root_login.name
  }
  rule = each.value
  arn  = aws_sns_topic.alerts.arn
}

data "aws_iam_policy_document" "alerts_topic" {
  statement {
    sid       = "EventBridgePublish"
    actions   = ["sns:Publish"]
    resources = [aws_sns_topic.alerts.arn]
    principals {
      type        = "Service"
      identifiers = ["events.amazonaws.com"]
    }
    condition {
      test     = "ArnEquals"
      variable = "aws:SourceArn"
      values = [
        aws_cloudwatch_event_rule.guardduty.arn,
        aws_cloudwatch_event_rule.root_login.arn,
      ]
    }
  }
}

resource "aws_sns_topic_policy" "alerts" {
  arn    = aws_sns_topic.alerts.arn
  policy = data.aws_iam_policy_document.alerts_topic.json
}
