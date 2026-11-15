# Terraform with the Grafana provider. Lints clean.

variable "prometheus_uid" {
  type = string
}

locals {
  common_labels = { team = "payments" }
}

resource "grafana_folder" "payments" {
  title = "Payments"
}

resource "grafana_rule_group" "payments" {
  name             = "payments"
  folder_uid       = grafana_folder.payments.uid
  interval_seconds = 60

  # Multiwindow, multi-burn-rate SLO alert done with Grafana math: one query
  # per window, plus a request-rate query as the traffic guard.
  rule {
    name           = "Payments availability SLO burn"
    condition      = "F"
    for            = "2m"
    no_data_state  = "NoData"
    exec_err_state = "Error"
    labels         = merge(local.common_labels, { severity = "page" })
    annotations = {
      summary     = "Payments is burning its 30-day error budget too fast"
      runbook_url = "https://runbooks.example.com/payments/slo-burn"
    }

    data {
      ref_id         = "A"
      datasource_uid = var.prometheus_uid
      relative_time_range {
        from = 3600
        to   = 0
      }
      model = jsonencode({
        expr    = "sum(rate(payment_requests_total{code=~\"5..\"}[1h])) / sum(rate(payment_requests_total[1h]))"
        instant = true
        range   = false
        refId   = "A"
      })
    }

    data {
      ref_id         = "B"
      datasource_uid = var.prometheus_uid
      relative_time_range {
        from = 300
        to   = 0
      }
      model = jsonencode({
        expr    = "sum(rate(payment_requests_total{code=~\"5..\"}[5m])) / sum(rate(payment_requests_total[5m]))"
        instant = true
        range   = false
        refId   = "B"
      })
    }

    data {
      ref_id         = "C"
      datasource_uid = var.prometheus_uid
      relative_time_range {
        from = 21600
        to   = 0
      }
      model = jsonencode({
        expr    = "sum(rate(payment_requests_total{code=~\"5..\"}[6h])) / sum(rate(payment_requests_total[6h]))"
        instant = true
        range   = false
        refId   = "C"
      })
    }

    data {
      ref_id         = "D"
      datasource_uid = var.prometheus_uid
      relative_time_range {
        from = 1800
        to   = 0
      }
      model = jsonencode({
        expr    = "sum(rate(payment_requests_total{code=~\"5..\"}[30m])) / sum(rate(payment_requests_total[30m]))"
        instant = true
        range   = false
        refId   = "D"
      })
    }

    data {
      ref_id         = "E"
      datasource_uid = var.prometheus_uid
      relative_time_range {
        from = 300
        to   = 0
      }
      model = jsonencode({
        expr    = "sum(rate(payment_requests_total[5m]))"
        instant = true
        range   = false
        refId   = "E"
      })
    }

    data {
      ref_id         = "F"
      datasource_uid = "__expr__"
      relative_time_range {
        from = 0
        to   = 0
      }
      model = jsonencode({
        type       = "math"
        expression = "(($A > 0.0144 && $B > 0.0144) || ($C > 0.006 && $D > 0.006)) && $E > 1"
        refId      = "F"
      })
    }
  }

  rule {
    name           = "Payments settlement stalled"
    condition      = "B"
    for            = "15m"
    no_data_state  = "NoData"
    exec_err_state = "Error"
    labels         = merge(local.common_labels, { severity = "page" })
    annotations = {
      summary     = "No settlement batch has succeeded for an hour"
      runbook_url = "https://runbooks.example.com/payments/settlement"
    }
    notification_settings {
      contact_point = "payments-oncall"
    }

    data {
      ref_id         = "A"
      datasource_uid = var.prometheus_uid
      relative_time_range {
        from = 300
        to   = 0
      }
      model = jsonencode({
        expr    = "max(time() - settlement_last_success_timestamp_seconds{service=\"payments\"})"
        instant = true
        range   = false
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
        type       = "threshold"
        expression = "A"
        conditions = [{ evaluator = { params = [3600], type = "gt" } }]
        refId      = "B"
      })
    }
  }
}
