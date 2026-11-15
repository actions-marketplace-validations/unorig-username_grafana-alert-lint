resource "grafana_rule_group" "api" {
  name             = "api"
  folder_uid       = grafana_folder.services.uid
  interval_seconds = 60

  # Fires (with --require-label team): no team label.
  rule {
    uid            = "gal008-no-team"
    name           = "API certificate expiring"
    condition      = "B"
    no_data_state  = "NoData"
    exec_err_state = "Error"
    for            = "1h"
    labels = {
      severity = "ticket"
    }
    annotations = {
      summary     = "The API TLS certificate expires within 7 days"
      runbook_url = "https://runbooks.example.com/api/tls"
    }

    data {
      ref_id         = "A"
      datasource_uid = "prometheus"
      relative_time_range {
        from = 300
        to   = 0
      }
      model = jsonencode({
        expr    = "min(probe_ssl_earliest_cert_expiry{job=\"api\"} - time())"
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
        conditions = [{ evaluator = { params = [604800], type = "lt" } }]
        refId      = "B"
      })
    }
  }

  # Fires twice (with --require-label team --require-label severity): no labels.
  rule {
    uid            = "gal008-no-labels"
    name           = "API certificate expiring unlabeled"
    condition      = "B"
    no_data_state  = "NoData"
    exec_err_state = "Error"
    for            = "1h"
    annotations = {
      summary     = "The API TLS certificate expires within 7 days"
      runbook_url = "https://runbooks.example.com/api/tls"
    }

    data {
      ref_id         = "A"
      datasource_uid = "prometheus"
      relative_time_range {
        from = 300
        to   = 0
      }
      model = jsonencode({
        expr    = "min(probe_ssl_earliest_cert_expiry{job=\"api\"} - time())"
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
        conditions = [{ evaluator = { params = [604800], type = "lt" } }]
        refId      = "B"
      })
    }
  }

  # Fires always: notification_settings with an empty receiver.
  rule {
    uid            = "gal008-empty-receiver"
    name           = "API certificate expiring direct"
    condition      = "B"
    no_data_state  = "NoData"
    exec_err_state = "Error"
    for            = "1h"
    labels = {
      severity = "ticket"
      team     = "api"
    }
    annotations = {
      summary     = "The API TLS certificate expires within 7 days"
      runbook_url = "https://runbooks.example.com/api/tls"
    }
    notification_settings {
      contact_point = ""
    }

    data {
      ref_id         = "A"
      datasource_uid = "prometheus"
      relative_time_range {
        from = 300
        to   = 0
      }
      model = jsonencode({
        expr    = "min(probe_ssl_earliest_cert_expiry{job=\"api\"} - time())"
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
        conditions = [{ evaluator = { params = [604800], type = "lt" } }]
        refId      = "B"
      })
    }
  }

  # Quiet: labels present and a receiver set.
  rule {
    uid            = "gal008-ok"
    name           = "API certificate expiring routed"
    condition      = "B"
    no_data_state  = "NoData"
    exec_err_state = "Error"
    for            = "1h"
    labels = {
      severity = "ticket"
      team     = "api"
    }
    annotations = {
      summary     = "The API TLS certificate expires within 7 days"
      runbook_url = "https://runbooks.example.com/api/tls"
    }
    notification_settings {
      contact_point = "api-oncall"
    }

    data {
      ref_id         = "A"
      datasource_uid = "prometheus"
      relative_time_range {
        from = 300
        to   = 0
      }
      model = jsonencode({
        expr    = "min(probe_ssl_earliest_cert_expiry{job=\"api\"} - time())"
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
        conditions = [{ evaluator = { params = [604800], type = "lt" } }]
        refId      = "B"
      })
    }
  }
}
