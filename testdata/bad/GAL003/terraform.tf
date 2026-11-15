resource "grafana_rule_group" "payments" {
  name             = "payments"
  folder_uid       = grafana_folder.services.uid
  interval_seconds = 60

  # Fires: Alerting pages on a datasource blip.
  rule {
    uid            = "gal003-alerting"
    name           = "Payments queue backlog"
    condition      = "B"
    no_data_state  = "NoData"
    exec_err_state = "Alerting"
    for            = "10m"
    labels = {
      severity = "page"
      team     = "payments"
    }
    annotations = {
      summary     = "The payments queue holds more than 500 jobs"
      runbook_url = "https://runbooks.example.com/payments/queue"
    }

    data {
      ref_id         = "A"
      datasource_uid = "prometheus"
      relative_time_range {
        from = 300
        to   = 0
      }
      model = jsonencode({
        expr    = "sum(job_queue_depth{service=\"payments\"})"
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
        conditions = [{ evaluator = { params = [500], type = "gt" } }]
        refId      = "B"
      })
    }
  }

  # Fires: OK hides query errors.
  rule {
    uid            = "gal003-ok"
    name           = "Payments worker saturation"
    condition      = "B"
    no_data_state  = "NoData"
    exec_err_state = "OK"
    for            = "10m"
    labels = {
      severity = "ticket"
      team     = "payments"
    }
    annotations = {
      summary     = "Payments workers are more than 90% busy"
      runbook_url = "https://runbooks.example.com/payments/workers"
    }

    data {
      ref_id         = "A"
      datasource_uid = "prometheus"
      relative_time_range {
        from = 300
        to   = 0
      }
      model = jsonencode({
        expr    = "avg(worker_busy_ratio{service=\"payments\"})"
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

  # Fires: not set, so it defaults to Alerting.
  rule {
    uid           = "gal003-unset"
    name          = "Payments settlement lag"
    condition     = "B"
    no_data_state = "NoData"
    for           = "5m"
    labels = {
      severity = "page"
      team     = "payments"
    }
    annotations = {
      summary     = "No settlement batch has succeeded for an hour"
      runbook_url = "https://runbooks.example.com/payments/settlement"
    }

    data {
      ref_id         = "A"
      datasource_uid = "prometheus"
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

  # Quiet: Error.
  rule {
    uid            = "gal003-error"
    name           = "Payments settlement lag error state"
    condition      = "B"
    no_data_state  = "NoData"
    exec_err_state = "Error"
    for            = "5m"
    labels = {
      severity = "page"
      team     = "payments"
    }
    annotations = {
      summary     = "No settlement batch has succeeded for an hour"
      runbook_url = "https://runbooks.example.com/payments/settlement"
    }

    data {
      ref_id         = "A"
      datasource_uid = "prometheus"
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
