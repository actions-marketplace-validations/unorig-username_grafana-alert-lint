resource "grafana_rule_group" "api" {
  name             = "api"
  folder_uid       = grafana_folder.services.uid
  interval_seconds = 60

  # Fires three times: windows under 4 scrapes (60s) or 4 subquery steps.
  rule {
    uid            = "gal005-short"
    name           = "API traffic dropped"
    condition      = "D"
    no_data_state  = "NoData"
    exec_err_state = "Error"
    for            = "10m"
    labels = {
      severity = "page"
      team     = "api"
    }
    annotations = {
      summary     = "The API is serving almost no traffic"
      runbook_url = "https://runbooks.example.com/api/traffic"
    }

    data {
      ref_id         = "A"
      datasource_uid = "prometheus"
      relative_time_range {
        from = 300
        to   = 0
      }
      model = jsonencode({
        expr    = "sum(rate(http_requests_total{service=\"api\"}[1m]))"
        instant = true
        range   = false
        refId   = "A"
      })
    }

    data {
      ref_id         = "B"
      datasource_uid = "prometheus"
      relative_time_range {
        from = 300
        to   = 0
      }
      model = jsonencode({
        expr    = "sum(increase(http_requests_total{service=\"api\"}[2m] offset 1h))"
        instant = true
        range   = false
        refId   = "B"
      })
    }

    data {
      ref_id         = "C"
      datasource_uid = "prometheus"
      relative_time_range {
        from = 300
        to   = 0
      }
      model = jsonencode({
        expr    = "sum(rate(http_requests_total{service=\"api\"}[2m:1m]))"
        instant = true
        range   = false
        refId   = "C"
      })
    }

    data {
      ref_id         = "D"
      datasource_uid = "__expr__"
      relative_time_range {
        from = 0
        to   = 0
      }
      model = jsonencode({
        type       = "math"
        expression = "$A < 1 || $B < 60 || $C < 1"
        refId      = "D"
      })
    }
  }

  # Quiet: long enough windows, macros, subqueries without a step, deriv.
  rule {
    uid            = "gal005-ok"
    name           = "API traffic dropped long windows"
    condition      = "E"
    no_data_state  = "NoData"
    exec_err_state = "Error"
    for            = "10m"
    labels = {
      severity = "page"
      team     = "api"
    }
    annotations = {
      summary     = "The API is serving almost no traffic"
      runbook_url = "https://runbooks.example.com/api/traffic"
    }

    data {
      ref_id         = "A"
      datasource_uid = "prometheus"
      relative_time_range {
        from = 300
        to   = 0
      }
      model = jsonencode({
        expr    = "sum(rate(http_requests_total{service=\"api\"}[5m] offset 1h))"
        instant = true
        range   = false
        refId   = "A"
      })
    }

    data {
      ref_id         = "B"
      datasource_uid = "prometheus"
      relative_time_range {
        from = 1800
        to   = 0
      }
      model = jsonencode({
        expr    = "max_over_time(sum(rate(http_requests_total{service=\"api\"}[5m]))[30m:1m])"
        instant = true
        range   = false
        refId   = "B"
      })
    }

    data {
      ref_id         = "C"
      datasource_uid = "prometheus"
      relative_time_range {
        from = 300
        to   = 0
      }
      model = jsonencode({
        expr    = "sum(rate(http_requests_total{service=\"api\"}[$__rate_interval]))"
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
        expr    = "sum(rate(http_requests_total{service=\"api\"}[10m:])) + deriv(process_resident_memory_bytes{job=\"api\"}[1m]) * 0"
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
        expression = "$A < 1 || $B < 1 || $C < 1 || $D < 1"
        refId      = "E"
      })
    }
  }
}
