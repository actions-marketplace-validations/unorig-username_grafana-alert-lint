resource "grafana_rule_group" "payments" {
  name             = "payments"
  folder_uid       = grafana_folder.services.uid
  interval_seconds = 60

  # Fires twice: no annotations at all.
  rule {
    uid            = "gal007-none"
    name           = "Payments refunds failing"
    condition      = "B"
    no_data_state  = "NoData"
    exec_err_state = "Error"
    for            = "5m"
    labels = {
      severity = "ticket"
      team     = "payments"
    }

    data {
      ref_id         = "A"
      datasource_uid = "prometheus"
      relative_time_range {
        from = 600
        to   = 0
      }
      model = jsonencode({
        expr    = "sum(increase(refund_failures_total{service=\"payments\"}[10m]))"
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
        conditions = [{ evaluator = { params = [5], type = "gt" } }]
        refId      = "B"
      })
    }
  }

  # Fires once (info): summary but no runbook_url.
  rule {
    uid            = "gal007-no-runbook"
    name           = "Payments refunds failing no runbook"
    condition      = "B"
    no_data_state  = "NoData"
    exec_err_state = "Error"
    for            = "5m"
    labels = {
      severity = "ticket"
      team     = "payments"
    }
    annotations = {
      summary = "More than 5 refunds failed in 10 minutes"
    }

    data {
      ref_id         = "A"
      datasource_uid = "prometheus"
      relative_time_range {
        from = 600
        to   = 0
      }
      model = jsonencode({
        expr    = "sum(increase(refund_failures_total{service=\"payments\"}[10m]))"
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
        conditions = [{ evaluator = { params = [5], type = "gt" } }]
        refId      = "B"
      })
    }
  }

  # Fires once (warning): summary is empty.
  rule {
    uid            = "gal007-empty-summary"
    name           = "Payments refunds failing empty summary"
    condition      = "B"
    no_data_state  = "NoData"
    exec_err_state = "Error"
    for            = "5m"
    labels = {
      severity = "ticket"
      team     = "payments"
    }
    annotations = {
      summary     = ""
      runbook_url = "https://runbooks.example.com/payments/refunds"
    }

    data {
      ref_id         = "A"
      datasource_uid = "prometheus"
      relative_time_range {
        from = 600
        to   = 0
      }
      model = jsonencode({
        expr    = "sum(increase(refund_failures_total{service=\"payments\"}[10m]))"
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
        conditions = [{ evaluator = { params = [5], type = "gt" } }]
        refId      = "B"
      })
    }
  }

  # Quiet: both set.
  rule {
    uid            = "gal007-ok"
    name           = "Payments refunds failing documented"
    condition      = "B"
    no_data_state  = "NoData"
    exec_err_state = "Error"
    for            = "5m"
    labels = {
      severity = "ticket"
      team     = "payments"
    }
    annotations = {
      summary     = "More than 5 refunds failed in 10 minutes"
      runbook_url = "https://runbooks.example.com/payments/refunds"
    }

    data {
      ref_id         = "A"
      datasource_uid = "prometheus"
      relative_time_range {
        from = 600
        to   = 0
      }
      model = jsonencode({
        expr    = "sum(increase(refund_failures_total{service=\"payments\"}[10m]))"
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
        conditions = [{ evaluator = { params = [5], type = "gt" } }]
        refId      = "B"
      })
    }
  }
}
