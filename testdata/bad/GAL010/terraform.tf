resource "grafana_rule_group" "api" {
  name             = "api"
  folder_uid       = grafana_folder.services.uid
  interval_seconds = 60

  # Fires: unbalanced parenthesis.
  rule {
    uid            = "gal010-syntax"
    name           = "API worker restarts"
    condition      = "B"
    no_data_state  = "NoData"
    exec_err_state = "Error"
    for            = "5m"
    labels = {
      severity = "ticket"
      team     = "api"
    }
    annotations = {
      summary     = "API containers restarted more than 3 times in 15 minutes"
      runbook_url = "https://runbooks.example.com/api/restarts"
    }

    data {
      ref_id         = "A"
      datasource_uid = "prometheus"
      relative_time_range {
        from = 900
        to   = 0
      }
      model = jsonencode({
        expr    = "sum(increase(kube_pod_container_status_restarts_total{container=\"api\"}[15m])"
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
        conditions = [{ evaluator = { params = [3], type = "gt" } }]
        refId      = "B"
      })
    }
  }

  # Fires: condition C does not exist.
  rule {
    uid            = "gal010-condition"
    name           = "API worker restarts bad condition"
    condition      = "C"
    no_data_state  = "NoData"
    exec_err_state = "Error"
    for            = "5m"
    labels = {
      severity = "ticket"
      team     = "api"
    }
    annotations = {
      summary     = "API containers restarted more than 3 times in 15 minutes"
      runbook_url = "https://runbooks.example.com/api/restarts"
    }

    data {
      ref_id         = "A"
      datasource_uid = "prometheus"
      relative_time_range {
        from = 900
        to   = 0
      }
      model = jsonencode({
        expr    = "sum(increase(kube_pod_container_status_restarts_total{container=\"api\"}[15m]))"
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
        conditions = [{ evaluator = { params = [3], type = "gt" } }]
        refId      = "B"
      })
    }
  }

  # Fires: no condition.
  rule {
    uid            = "gal010-no-condition"
    name           = "API worker restarts no condition"
    no_data_state  = "NoData"
    exec_err_state = "Error"
    for            = "5m"
    labels = {
      severity = "ticket"
      team     = "api"
    }
    annotations = {
      summary     = "API containers restarted more than 3 times in 15 minutes"
      runbook_url = "https://runbooks.example.com/api/restarts"
    }

    data {
      ref_id         = "A"
      datasource_uid = "prometheus"
      relative_time_range {
        from = 900
        to   = 0
      }
      model = jsonencode({
        expr    = "sum(increase(kube_pod_container_status_restarts_total{container=\"api\"}[15m]))"
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
        conditions = [{ evaluator = { params = [3], type = "gt" } }]
        refId      = "B"
      })
    }
  }

  # Quiet: Loki queries are not PromQL.
  rule {
    uid            = "gal010-loki"
    name           = "API panics in logs"
    condition      = "C"
    no_data_state  = "NoData"
    exec_err_state = "Error"
    for            = "5m"
    labels = {
      severity = "ticket"
      team     = "api"
    }
    annotations = {
      summary     = "The API logged a panic"
      runbook_url = "https://runbooks.example.com/api/panics"
    }

    data {
      ref_id         = "A"
      datasource_uid = "loki"
      relative_time_range {
        from = 300
        to   = 0
      }
      model = jsonencode({
        expr      = "sum(count_over_time({app=\"api\"} |= \"panic\" [5m]))"
        queryType = "instant"
        refId     = "A"
      })
    }

    data {
      ref_id         = "B"
      datasource_uid = "loki"
      relative_time_range {
        from = 300
        to   = 0
      }
      model = jsonencode({
        datasource = { type = "loki", uid = "loki" }
        expr       = "sum(rate({app=\"api\"}[1m]))"
        queryType  = "instant"
        refId      = "B"
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
        expression = "$A > 0 && $B > 1"
        refId      = "C"
      })
    }
  }
}
