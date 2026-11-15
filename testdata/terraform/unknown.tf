# Every field the checks need comes from a variable, local, data source or
# function we cannot evaluate. The linter must not crash and must not warn.

locals {
  common_labels = { severity = "page" }
  model         = { expr = "sum(rate(http_requests_total[1m]))", instant = true, refId = "A" }
}

resource "grafana_rule_group" "checkout" {
  for_each         = toset(var.services)
  name             = each.key
  folder_uid       = grafana_folder.services.uid
  interval_seconds = 60

  rule {
    name           = "${each.key} error ratio"
    condition      = local.condition
    for            = var.pending_period
    no_data_state  = var.no_data_state
    exec_err_state = local.exec_err_state
    labels         = merge(local.common_labels, { team = var.team })
    annotations    = local.annotations
    notification_settings {
      contact_point = var.contact_point
    }

    data {
      ref_id         = "A"
      datasource_uid = data.grafana_data_source.prometheus.uid
      relative_time_range {
        from = 600
        to   = 0
      }
      model = jsonencode({
        expr    = format("sum(rate(%s{code=~\"5..\"}[1m])) / sum(rate(%s[1m]))", var.metric, var.metric)
        instant = var.instant
        range   = var.range
        refId   = "A"
      })
    }

    data {
      ref_id         = "B"
      datasource_uid = "__expr__"
      relative_time_range {
        from = 0
        to   = 0
      }
      model = jsonencode({
        type       = "reduce"
        expression = "A"
        reducer    = var.reducer
        refId      = "B"
      })
    }

    dynamic "data" {
      for_each = var.extra_queries
      content {
        ref_id         = data.value.ref_id
        datasource_uid = data.value.datasource_uid
        model          = jsonencode(data.value.model)
      }
    }
  }

  # The ratio is known, but the condition is not, so the guard may be there.
  rule {
    name           = "Checkout error ratio"
    condition      = var.condition
    for            = "5m"
    no_data_state  = "NoData"
    exec_err_state = "Error"
    labels         = { team = var.team, severity = "page" }
    annotations = {
      summary     = var.summary
      runbook_url = "https://runbooks.example.com/checkout/errors"
    }

    data {
      ref_id         = "A"
      datasource_uid = var.prometheus_uid
      relative_time_range {
        from = 600
        to   = 0
      }
      model = jsonencode({
        expr    = "sum(rate(http_requests_total{service=\"checkout\",code=~\"5..\"}[5m])) / sum(rate(http_requests_total{service=\"checkout\"}[5m]))"
        instant = true
        range   = false
        refId   = "A"
      })
    }

    data {
      ref_id         = "B"
      datasource_uid = var.prometheus_uid
      relative_time_range {
        from = 600
        to   = 0
      }
      model = jsonencode({
        expr    = var.traffic_expr
        instant = true
        range   = false
        refId   = "B"
      })
    }

    data {
      ref_id         = "C"
      datasource_uid = "__expr__"
      relative_time_range {
        from = 0
        to   = 0
      }
      model = jsonencode({
        type       = "math"
        expression = var.math
        refId      = "C"
      })
    }
  }

  # Whole models we cannot read: a local, and a function we do not evaluate.
  rule {
    name           = "Checkout latency"
    condition      = "C"
    for            = "5m"
    no_data_state  = "NoData"
    exec_err_state = "Error"
    labels         = { team = "checkout", severity = "page" }
    annotations = {
      summary     = "Checkout p99 latency is above 500ms"
      runbook_url = "https://runbooks.example.com/checkout/latency"
    }
    dynamic "notification_settings" {
      for_each = var.contact_point == null ? [] : [1]
      content {
        contact_point = var.contact_point
      }
    }

    data {
      ref_id         = "A"
      datasource_uid = "prometheus"
      relative_time_range {
        from = 600
        to   = 0
      }
      model = local.model
    }

    data {
      ref_id         = "B"
      datasource_uid = "prometheus"
      relative_time_range {
        from = 600
        to   = 0
      }
      model = file("${path.module}/latency.json")
    }

    data {
      ref_id         = "C"
      datasource_uid = "__expr__"
      relative_time_range {
        from = 0
        to   = 0
      }
      model = jsonencode({
        type       = "math"
        expression = "$A > 0.5 && $B > 1"
        refId      = "C"
      })
    }
  }

  dynamic "rule" {
    for_each = var.generated_rules
    content {
      name      = rule.value.name
      condition = "A"
    }
  }
}
