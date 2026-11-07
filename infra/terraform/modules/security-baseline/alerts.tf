# Alerts by email: root activity, changes an intruder makes first, optional GuardDuty findings, and
# optional cost anomalies. EventBridge rules on AWS service events and SNS email are free.
# (The monthly budget lives in modules/budget.)

locals {
  alert_rules = {
    # The member account's root credentials are deleted, so any root activity is suspicious.
    root_activity = {
      description = "Any console sign-in or API call made as the root user"
      pattern = {
        "detail-type" = ["AWS Console Sign In via CloudTrail", "AWS API Call via CloudTrail"]
        detail        = { userIdentity = { type = ["Root"] } }
      }
    }
    # Without GuardDuty, these catch the usual first moves after a credential leak: long-lived access
    # for the intruder, and switching off the audit log. Your own Terraform changes to the trail
    # trigger it too, which is the intended "was that you?" check.
    sensitive_changes = {
      description = "IAM users or access keys created, CloudTrail stopped or changed, leaving the organization"
      pattern = {
        "detail-type" = ["AWS API Call via CloudTrail"]
        detail = {
          eventSource = ["iam.amazonaws.com", "cloudtrail.amazonaws.com", "organizations.amazonaws.com"]
          eventName = [
            "CreateUser", "CreateAccessKey", "CreateLoginProfile", "AttachUserPolicy", "PutUserPolicy",
            "StopLogging", "DeleteTrail", "UpdateTrail", "PutEventSelectors",
            "LeaveOrganization",
          ]
        }
      }
    }
  }
}

# Not encrypted at rest: EventBridge cannot publish to a topic encrypted with the free AWS-managed
# key, and a customer-managed key costs $1/month. Alerts carry event metadata, not financial data.
resource "aws_sns_topic" "alerts" {
  name = "${var.name}-security-alerts"
}

resource "aws_sns_topic_subscription" "alerts_email" {
  topic_arn = aws_sns_topic.alerts.arn
  protocol  = "email"
  endpoint  = var.alert_email
}

resource "aws_cloudwatch_event_rule" "alert" {
  for_each      = local.alert_rules
  name          = "${var.name}-${replace(each.key, "_", "-")}"
  description   = each.value.description
  event_pattern = jsonencode(each.value.pattern)
}

resource "aws_cloudwatch_event_target" "alert" {
  for_each = aws_cloudwatch_event_rule.alert
  rule     = each.value.name
  arn      = aws_sns_topic.alerts.arn
}

# IAM, Organizations and root console sign-ins are recorded in us-east-1 whatever the home region,
# so forward matching events to the home region's default bus, where the rules above send them on.
resource "aws_cloudwatch_event_rule" "forward" {
  for_each      = { for k, v in local.alert_rules : k => v if local.forward_global_events }
  provider      = aws.us_east_1
  name          = "${var.name}-${replace(each.key, "_", "-")}-forward"
  description   = "Forwards to ${local.region}: ${each.value.description}"
  event_pattern = jsonencode(each.value.pattern)
}

resource "aws_cloudwatch_event_target" "forward" {
  for_each = aws_cloudwatch_event_rule.forward
  provider = aws.us_east_1
  rule     = each.value.name
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

# --- GuardDuty (opt-in: free for 30 days, then billed by analysed events) ------------------------

resource "aws_guardduty_detector" "this" {
  count                        = var.enable_guardduty ? 1 : 0
  enable                       = true
  finding_publishing_frequency = "FIFTEEN_MINUTES"
}

resource "aws_cloudwatch_event_rule" "guardduty" {
  count       = var.enable_guardduty ? 1 : 0
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

resource "aws_cloudwatch_event_target" "guardduty" {
  count = var.enable_guardduty ? 1 : 0
  rule  = aws_cloudwatch_event_rule.guardduty[0].name
  arn   = aws_sns_topic.alerts.arn
}

# --- SNS topic policy -------------------------------------------------------------------------------

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
      values = concat(
        [for r in aws_cloudwatch_event_rule.alert : r.arn],
        aws_cloudwatch_event_rule.guardduty[*].arn,
      )
    }
  }
}

resource "aws_sns_topic_policy" "alerts" {
  arn    = aws_sns_topic.alerts.arn
  policy = data.aws_iam_policy_document.alerts_topic.json
}

# --- Cost anomaly detection (free; only one services monitor is allowed per account) --------------

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
