resource "grafana_rule_group" "api" {
  name             = "api"
  folder_uid       = grafana_folder.services.uid
  interval_seconds = 60

  # Fires: Alerting pages when the scrape breaks.
  rule {
    uid            = "gal002-alerting"
    name           = "API instance down"
    condition      = "B"
    no_data_state  = "Alerting"
    exec_err_state = "Error"
    for            = "5m"
    labels = {
      severity = "page"
      team     = "api"
    }
    annotations = {
      summary     = "An API instance is not being scraped"
      runbook_url = "https://runbooks.example.com/api/down"
    }

    data {
      ref_id         = "A"
      datasource_uid = "prometheus"
      relative_time_range {
        from = 300
        to   = 0
      }
      model = jsonencode({
        expr    = "min by (instance) (up{job=\"api\"})"
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
        conditions = [{ evaluator = { params = [1], type = "lt" } }]
        refId      = "B"
      })
    }
  }

  # Fires: OK on a query that always returns series while healthy.
  rule {
    uid            = "gal002-ok"
    name           = "API queue backlog"
    condition      = "B"
    no_data_state  = "OK"
    exec_err_state = "Error"
    for            = "10m"
    labels = {
      severity = "ticket"
      team     = "api"
    }
    annotations = {
      summary     = "The API job queue holds more than 1000 jobs"
      runbook_url = "https://runbooks.example.com/api/queue"
    }

    data {
      ref_id         = "A"
      datasource_uid = "prometheus"
      relative_time_range {
        from = 300
        to   = 0
      }
      model = jsonencode({
        expr    = "sum(job_queue_depth{service=\"api\"})"
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
        conditions = [{ evaluator = { params = [1000], type = "gt" } }]
        refId      = "B"
      })
    }
  }

  # Quiet: the query filters, so no data is the healthy state.
  rule {
    uid            = "gal002-ok-filter"
    name           = "API queue backlog filtered"
    condition      = "B"
    no_data_state  = "OK"
    exec_err_state = "Error"
    for            = "10m"
    labels = {
      severity = "ticket"
      team     = "api"
    }
    annotations = {
      summary     = "The API job queue holds more than 1000 jobs"
      runbook_url = "https://runbooks.example.com/api/queue"
    }

    data {
      ref_id         = "A"
      datasource_uid = "prometheus"
      relative_time_range {
        from = 300
        to   = 0
      }
      model = jsonencode({
        expr    = "sum(job_queue_depth{service=\"api\"}) > 1000"
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

  # Quiet: absent() returns nothing while the target is up.
  rule {
    uid            = "gal002-ok-absent"
    name           = "API target missing"
    condition      = "B"
    no_data_state  = "OK"
    exec_err_state = "Error"
    for            = "5m"
    labels = {
      severity = "page"
      team     = "api"
    }
    annotations = {
      summary     = "Prometheus has no API target"
      runbook_url = "https://runbooks.example.com/api/down"
    }

    data {
      ref_id         = "A"
      datasource_uid = "prometheus"
      relative_time_range {
        from = 300
        to   = 0
      }
      model = jsonencode({
        expr    = "absent(up{job=\"api\"})"
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

  # Quiet: KeepLast.
  rule {
    uid            = "gal002-keeplast"
    name           = "API memory high"
    condition      = "B"
    no_data_state  = "KeepLast"
    exec_err_state = "KeepLast"
    for            = "15m"
    labels = {
      severity = "ticket"
      team     = "api"
    }
    annotations = {
      summary     = "An API pod uses more than 90% of its memory limit"
      runbook_url = "https://runbooks.example.com/api/memory"
    }

    data {
      ref_id         = "A"
      datasource_uid = "prometheus"
      relative_time_range {
        from = 300
        to   = 0
      }
      model = jsonencode({
        expr    = "max by (pod) (container_memory_working_set_bytes{container=\"api\"} / container_spec_memory_limit_bytes{container=\"api\"})"
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
        conditions = [{ evaluator = { params = [0.9], type = "gt" } }]
        refId      = "B"
      })
    }
  }
}
