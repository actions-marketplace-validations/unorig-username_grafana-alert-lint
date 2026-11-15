resource "grafana_rule_group" "checkout" {
  name             = "checkout"
  folder_uid       = grafana_folder.services.uid
  interval_seconds = 60

  # Fires three times: each aggregation drops le.
  rule {
    uid            = "gal006-drops-le"
    name           = "Checkout latency by service"
    condition      = "E"
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
        expr    = "histogram_quantile(0.99, sum by (service) (rate(http_request_duration_seconds_bucket{service=\"checkout\"}[5m])))"
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
        expr    = "histogram_quantile(0.99, sum without (le, instance) (rate(http_request_duration_seconds_bucket{service=\"checkout\"}[5m])))"
        instant = true
        range   = false
        refId   = "B"
      })
    }

    data {
      ref_id         = "C"
      datasource_uid = "prometheus"
      relative_time_range {
        from = 600
        to   = 0
      }
      model = jsonencode({
        expr    = "histogram_quantile(0.99, sum(rate(http_request_duration_seconds_bucket{service=\"checkout\"}[5m])))"
        instant = true
        range   = false
        refId   = "C"
      })
    }

    data {
      ref_id         = "D"
      datasource_uid = "prometheus"
      relative_time_range {
        from = 600
        to   = 0
      }
      model = jsonencode({
        expr    = "sum(rate(http_request_duration_seconds_count{service=\"checkout\"}[5m]))"
        instant = true
        range   = false
        refId   = "D"
      })
    }

    data {
      ref_id         = "E"
      datasource_uid = "__expr__"
      relative_time_range {
        from = 0
        to   = 0
      }
      model = jsonencode({
        type       = "math"
        expression = "($A > 0.5 || $B > 0.5 || $C > 0.5) && $D > 1"
        refId      = "E"
      })
    }
  }

  # Quiet: le kept, no aggregation, or a native histogram.
  rule {
    uid            = "gal006-ok"
    name           = "Checkout latency keeps le"
    condition      = "F"
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
        expr    = "histogram_quantile(0.99, sum by (le, service) (rate(http_request_duration_seconds_bucket{service=\"checkout\"}[5m])))"
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
        expr    = "histogram_quantile(0.99, sum without (instance, pod) (rate(http_request_duration_seconds_bucket{service=\"checkout\"}[5m])))"
        instant = true
        range   = false
        refId   = "B"
      })
    }

    data {
      ref_id         = "C"
      datasource_uid = "prometheus"
      relative_time_range {
        from = 600
        to   = 0
      }
      model = jsonencode({
        expr    = "histogram_quantile(0.99, rate(http_request_duration_seconds_bucket{service=\"checkout\"}[5m]))"
        instant = true
        range   = false
        refId   = "C"
      })
    }

    data {
      ref_id         = "D"
      datasource_uid = "prometheus"
      relative_time_range {
        from = 600
        to   = 0
      }
      model = jsonencode({
        expr    = "histogram_quantile(0.99, sum by (service) (rate(http_request_duration_seconds{service=\"checkout\"}[5m])))"
        instant = true
        range   = false
        refId   = "D"
      })
    }

    data {
      ref_id         = "E"
      datasource_uid = "prometheus"
      relative_time_range {
        from = 600
        to   = 0
      }
      model = jsonencode({
        expr    = "sum(rate(http_request_duration_seconds_count{service=\"checkout\"}[5m]))"
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
        expression = "($A > 0.5 || $B > 0.5 || $C > 0.5 || $D > 0.5) && $E > 1"
        refId      = "F"
      })
    }
  }
}
