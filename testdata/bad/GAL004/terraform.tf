resource "grafana_rule_group" "checkout" {
  name             = "checkout"
  folder_uid       = grafana_folder.services.uid
  interval_seconds = 60

  # Fires: for is not set.
  rule {
    uid            = "gal004-unset"
    name           = "Checkout cart service saturated"
    condition      = "B"
    no_data_state  = "NoData"
    exec_err_state = "Error"
    labels = {
      severity = "page"
      team     = "checkout"
    }
    annotations = {
      summary     = "Cart workers are more than 90% busy"
      runbook_url = "https://runbooks.example.com/checkout/cart"
    }

    data {
      ref_id         = "A"
      datasource_uid = "prometheus"
      relative_time_range {
        from = 300
        to   = 0
      }
      model = jsonencode({
        expr    = "avg(worker_busy_ratio{service=\"cart\"})"
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

  # Fires: for is 0s.
  rule {
    uid            = "gal004-zero"
    name           = "Checkout cart service saturated now"
    condition      = "B"
    no_data_state  = "NoData"
    exec_err_state = "Error"
    for            = "0s"
    labels = {
      severity = "page"
      team     = "checkout"
    }
    annotations = {
      summary     = "Cart workers are more than 90% busy"
      runbook_url = "https://runbooks.example.com/checkout/cart"
    }

    data {
      ref_id         = "A"
      datasource_uid = "prometheus"
      relative_time_range {
        from = 300
        to   = 0
      }
      model = jsonencode({
        expr    = "avg(worker_busy_ratio{service=\"cart\"})"
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

  # Fires: for is an empty string.
  rule {
    uid            = "gal004-empty"
    name           = "Checkout cart service saturated empty"
    condition      = "B"
    no_data_state  = "NoData"
    exec_err_state = "Error"
    for            = ""
    labels = {
      severity = "page"
      team     = "checkout"
    }
    annotations = {
      summary     = "Cart workers are more than 90% busy"
      runbook_url = "https://runbooks.example.com/checkout/cart"
    }

    data {
      ref_id         = "A"
      datasource_uid = "prometheus"
      relative_time_range {
        from = 300
        to   = 0
      }
      model = jsonencode({
        expr    = "avg(worker_busy_ratio{service=\"cart\"})"
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

  # Quiet: 1m30s.
  rule {
    uid            = "gal004-set"
    name           = "Checkout cart service saturated with pending period"
    condition      = "B"
    no_data_state  = "NoData"
    exec_err_state = "Error"
    for            = "1m30s"
    labels = {
      severity = "page"
      team     = "checkout"
    }
    annotations = {
      summary     = "Cart workers are more than 90% busy"
      runbook_url = "https://runbooks.example.com/checkout/cart"
    }

    data {
      ref_id         = "A"
      datasource_uid = "prometheus"
      relative_time_range {
        from = 300
        to   = 0
      }
      model = jsonencode({
        expr    = "avg(worker_busy_ratio{service=\"cart\"})"
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
