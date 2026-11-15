resource "grafana_rule_group" "checkout" {
  name             = "checkout"
  folder_uid       = grafana_folder.services.uid
  interval_seconds = 60

  # Fires: a plain error ratio with no traffic guard.
  rule {
    uid            = "gal001-ratio"
    name           = "Checkout error ratio"
    condition      = "B"
    no_data_state  = "NoData"
    exec_err_state = "Error"
    for            = "5m"
    labels = {
      severity = "page"
      team     = "checkout"
    }
    annotations = {
      summary     = "Checkout fails more than 5% of requests"
      runbook_url = "https://runbooks.example.com/checkout/errors"
    }

    data {
      ref_id         = "A"
      datasource_uid = "prometheus"
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
      datasource_uid = "__expr__"
      relative_time_range {
        from = 0
        to   = 0
      }
      model = jsonencode({
        type       = "threshold"
        expression = "A"
        conditions = [{ evaluator = { params = [0.05], type = "gt" } }]
        refId      = "B"
      })
    }
  }

  # Fires: p99 latency with no traffic guard.
  rule {
    uid            = "gal001-quantile"
    name           = "Checkout p99 latency"
    condition      = "B"
    no_data_state  = "NoData"
    exec_err_state = "Error"
    for            = "5m"
    labels = {
      severity = "page"
      team     = "checkout"
    }
    annotations = {
      summary     = "Checkout p99 latency is above 500ms"
      runbook_url = "https://runbooks.example.com/checkout/latency"
    }

    data {
      ref_id         = "A"
      datasource_uid = "prometheus"
      relative_time_range {
        from = 600
        to   = 0
      }
      model = jsonencode({
        expr    = "histogram_quantile(0.99, sum by (le) (rate(http_request_duration_seconds_bucket{service=\"checkout\"}[5m])))"
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
        conditions = [{ evaluator = { params = [0.5], type = "gt" } }]
        refId      = "B"
      })
    }
  }

  # Fires: the other side of "and" is another ratio, not a traffic guard.
  rule {
    uid            = "gal001-two-windows"
    name           = "Checkout error budget burn"
    condition      = "B"
    no_data_state  = "OK"
    exec_err_state = "Error"
    for            = "2m"
    labels = {
      severity = "page"
      team     = "checkout"
    }
    annotations = {
      summary     = "Checkout is burning its error budget 14.4 times too fast"
      runbook_url = "https://runbooks.example.com/checkout/slo"
    }

    data {
      ref_id         = "A"
      datasource_uid = "prometheus"
      relative_time_range {
        from = 3600
        to   = 0
      }
      model = jsonencode({
        expr    = <<-EOT
          sum(rate(http_requests_total{service="checkout",code=~"5.."}[1h])) / sum(rate(http_requests_total{service="checkout"}[1h])) > 0.0144
          and
          sum(rate(http_requests_total{service="checkout",code=~"5.."}[5m])) / sum(rate(http_requests_total{service="checkout"}[5m])) > 0.0144
        EOT
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
        conditions = [{ evaluator = { params = [0], type = "gt" } }]
        refId      = "B"
      })
    }
  }

  # Quiet: guarded in PromQL with "and" on request rate.
  rule {
    uid            = "gal001-guarded-and"
    name           = "Checkout error ratio with guard"
    condition      = "B"
    no_data_state  = "OK"
    exec_err_state = "Error"
    for            = "5m"
    labels = {
      severity = "page"
      team     = "checkout"
    }
    annotations = {
      summary     = "Checkout fails more than 5% of requests"
      runbook_url = "https://runbooks.example.com/checkout/errors"
    }

    data {
      ref_id         = "A"
      datasource_uid = "prometheus"
      relative_time_range {
        from = 600
        to   = 0
      }
      model = jsonencode({
        expr    = <<-EOT
          (
            sum(rate(http_requests_total{service="checkout",code=~"5.."}[5m]))
              /
            sum(rate(http_requests_total{service="checkout"}[5m]))
          )
          and on ()
          sum(rate(http_requests_total{service="checkout"}[5m])) > 1
        EOT
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
        conditions = [{ evaluator = { params = [0.05], type = "gt" } }]
        refId      = "B"
      })
    }
  }

  # Quiet: clamp_min on the denominator.
  rule {
    uid            = "gal001-clamp-min"
    name           = "Checkout error ratio with clamp_min"
    condition      = "B"
    no_data_state  = "NoData"
    exec_err_state = "Error"
    for            = "5m"
    labels = {
      severity = "page"
      team     = "checkout"
    }
    annotations = {
      summary     = "Checkout fails more than 5% of requests"
      runbook_url = "https://runbooks.example.com/checkout/errors"
    }

    data {
      ref_id         = "A"
      datasource_uid = "prometheus"
      relative_time_range {
        from = 600
        to   = 0
      }
      model = jsonencode({
        expr    = "sum(rate(http_requests_total{service=\"checkout\",code=~\"5..\"}[5m])) / clamp_min(sum(rate(http_requests_total{service=\"checkout\"}[5m])), 1)"
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
        conditions = [{ evaluator = { params = [0.05], type = "gt" } }]
        refId      = "B"
      })
    }
  }

  # Quiet: the condition combines the ratio with a request-rate query.
  rule {
    uid            = "gal001-guarded-math"
    name           = "Checkout p99 latency with traffic query"
    condition      = "C"
    no_data_state  = "NoData"
    exec_err_state = "Error"
    for            = "5m"
    labels = {
      severity = "page"
      team     = "checkout"
    }
    annotations = {
      summary     = "Checkout p99 latency is above 500ms"
      runbook_url = "https://runbooks.example.com/checkout/latency"
    }

    data {
      ref_id         = "A"
      datasource_uid = "prometheus"
      relative_time_range {
        from = 600
        to   = 0
      }
      model = jsonencode({
        expr    = "histogram_quantile(0.99, sum by (le) (rate(http_request_duration_seconds_bucket{service=\"checkout\"}[5m])))"
        instant = true
        range   = false
        refId   = "A"
      })
    }

    data {
      ref_id         = "B"
      datasource_uid = "prometheus"
      relative_time_range {
        from = 600
        to   = 0
      }
      model = jsonencode({
        expr    = "sum(rate(http_request_duration_seconds_count{service=\"checkout\"}[5m]))"
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
        expression = "$A > 0.5 && $B > 1"
        refId      = "C"
      })
    }
  }

  # Quiet: a ratio of gauges is not a ratio of rates.
  rule {
    uid            = "gal001-gauge-ratio"
    name           = "Checkout disk almost full"
    condition      = "B"
    no_data_state  = "NoData"
    exec_err_state = "Error"
    for            = "15m"
    labels = {
      severity = "ticket"
      team     = "checkout"
    }
    annotations = {
      summary     = "A checkout node has less than 10% disk left"
      runbook_url = "https://runbooks.example.com/checkout/disk"
    }

    data {
      ref_id         = "A"
      datasource_uid = "prometheus"
      relative_time_range {
        from = 600
        to   = 0
      }
      model = jsonencode({
        expr    = "min by (instance) (node_filesystem_avail_bytes{job=\"checkout\"} / node_filesystem_size_bytes{job=\"checkout\"})"
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
        conditions = [{ evaluator = { params = [0.1], type = "lt" } }]
        refId      = "B"
      })
    }
  }
}
