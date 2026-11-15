resource "grafana_rule_group" "checkout" {
  name             = "checkout"
  folder_uid       = grafana_folder.services.uid
  interval_seconds = 60

  # Fires: a range query reduced with last.
  rule {
    uid            = "gal009-range-last"
    name           = "Checkout open orders high"
    condition      = "C"
    no_data_state  = "NoData"
    exec_err_state = "Error"
    for            = "5m"
    labels = {
      severity = "ticket"
      team     = "checkout"
    }
    annotations = {
      summary     = "More than 5000 checkout orders are open"
      runbook_url = "https://runbooks.example.com/checkout/orders"
    }

    data {
      ref_id         = "A"
      datasource_uid = "prometheus"
      relative_time_range {
        from = 600
        to   = 0
      }
      model = jsonencode({
        expr    = "sum(checkout_open_orders)"
        instant = false
        range   = true
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
        reducer    = "last"
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
        type       = "threshold"
        expression = "B"
        conditions = [{ evaluator = { params = [5000], type = "gt" } }]
        refId      = "C"
      })
    }
  }

  # Fires: instant false, classic condition on last.
  rule {
    uid            = "gal009-classic-last"
    name           = "Checkout open orders high classic"
    condition      = "B"
    no_data_state  = "NoData"
    exec_err_state = "Error"
    for            = "5m"
    labels = {
      severity = "ticket"
      team     = "checkout"
    }
    annotations = {
      summary     = "More than 5000 checkout orders are open"
      runbook_url = "https://runbooks.example.com/checkout/orders"
    }

    data {
      ref_id         = "A"
      datasource_uid = "prometheus"
      relative_time_range {
        from = 600
        to   = 0
      }
      model = jsonencode({
        expr    = "sum(checkout_open_orders)"
        instant = false
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
        type       = "classic_conditions"
        conditions = [{ evaluator = { params = [5000], type = "gt" }, operator = { type = "and" }, query = { params = ["A"] }, reducer = { type = "last" }, type = "query" }]
        refId      = "B"
      })
    }
  }

  # Quiet: a range query reduced with mean needs the range.
  rule {
    uid            = "gal009-range-mean"
    name           = "Checkout open orders high on average"
    condition      = "C"
    no_data_state  = "NoData"
    exec_err_state = "Error"
    for            = "5m"
    labels = {
      severity = "ticket"
      team     = "checkout"
    }
    annotations = {
      summary     = "More than 5000 checkout orders are open on average"
      runbook_url = "https://runbooks.example.com/checkout/orders"
    }

    data {
      ref_id         = "A"
      datasource_uid = "prometheus"
      relative_time_range {
        from = 600
        to   = 0
      }
      model = jsonencode({
        expr    = "sum(checkout_open_orders)"
        instant = false
        range   = true
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
        reducer    = "mean"
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
        type       = "threshold"
        expression = "B"
        conditions = [{ evaluator = { params = [5000], type = "gt" } }]
        refId      = "C"
      })
    }
  }

  # Quiet: an instant query reduced with last.
  rule {
    uid            = "gal009-instant"
    name           = "Checkout open orders high instant"
    condition      = "C"
    no_data_state  = "NoData"
    exec_err_state = "Error"
    for            = "5m"
    labels = {
      severity = "ticket"
      team     = "checkout"
    }
    annotations = {
      summary     = "More than 5000 checkout orders are open"
      runbook_url = "https://runbooks.example.com/checkout/orders"
    }

    data {
      ref_id         = "A"
      datasource_uid = "prometheus"
      relative_time_range {
        from = 600
        to   = 0
      }
      model = jsonencode({
        expr    = "sum(checkout_open_orders)"
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
        type       = "reduce"
        expression = "A"
        reducer    = "last"
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
        type       = "threshold"
        expression = "B"
        conditions = [{ evaluator = { params = [5000], type = "gt" } }]
        refId      = "C"
      })
    }
  }
}
