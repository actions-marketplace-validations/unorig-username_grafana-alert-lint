# A model written as a heredoc JSON string instead of jsonencode().
resource "grafana_rule_group" "api" {
  name             = "api"
  folder_uid       = "services"
  interval_seconds = 60

  rule {
    name           = "API traffic dropped"
    condition      = "B"
    for            = "10m"
    no_data_state  = "NoData"
    exec_err_state = "Error"
    labels         = { team = "api", severity = "page" }
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
      model = <<EOT
{
  "refId": "A",
  "instant": true,
  "expr": "sum(rate(http_requests_total{service=\"api\"}[1m]))"
}
EOT
    }

    data {
      ref_id         = "B"
      datasource_uid = "__expr__"
      relative_time_range {
        from = 0
        to   = 0
      }
      model = "{\"type\": \"threshold\", \"expression\": \"A\", \"conditions\": [{\"evaluator\": {\"params\": [1], \"type\": \"lt\"}}], \"refId\": \"B\"}"
    }
  }
}
