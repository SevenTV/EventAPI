resource "kubernetes_namespace" "app" {
  metadata {
    name = var.namespace
  }
}

resource "kubernetes_secret" "app" {
  metadata {
    name      = "eventapi"
    namespace = var.namespace
  }

  data = {
    "config.yaml" = templatefile("${path.module}/config.template.yaml", {
      redis_address      = local.infra.redis_host,
      redis_username     = "default",
      redis_password     = local.infra.redis_password,
      nats_url           = "nats.database.svc.cluster.local:4222"
      nats_subject       = var.nats_events_subject
      bind               = "0.0.0.0:3000",
      heartbeat_interval = tostring(var.heartbeat_interval),
      subscription_limit = tostring(var.subscription_limit),
      connection_limit   = tostring(var.connection_limit),
      ttl                = tostring(var.ttl),
      bridge_url         = "http://api.app.svc.cluster.local:9700",
    })
  }
}

resource "kubernetes_deployment" "app" {
  metadata {
    name      = "eventapi"
    namespace = kubernetes_namespace.app.metadata[0].name
    labels    = {
      app = "eventapi"
    }
  }

  timeouts {
    create = "4m"
    update = "8m"
    delete = "2m"
  }

  spec {
    selector {
      match_labels = {
        app = "eventapi"
      }
    }

    strategy {
      rolling_update {
        max_surge       = "2"
        max_unavailable = "2"
      }
      type = "RollingUpdate"
    }

    template {
      metadata {
        labels = {
          app = "eventapi"
        }
      }

      spec {
        node_selector = {
          "7tv.io/node-pool" = "arm"
        }

        toleration {
          key      = "7tv.io/node-pool"
          operator = "Equal"
          value    = "arm"
          effect   = "NoSchedule"
        }

        container {
          name  = "eventapi"
          image = local.image_url

          port {
            name           = "http"
            container_port = 3000
            protocol       = "TCP"
          }

          port {
            name           = "metrics"
            container_port = 9100
            protocol       = "TCP"
          }

          port {
            name           = "health"
            container_port = 9200
            protocol       = "TCP"
          }

          port {
            name           = "pprof"
            container_port = 9300
            protocol       = "TCP"
          }

          env {
            name = "EVENTS_K8S_POD_NAME"
            value_from {
              field_ref {
                field_path = "metadata.name"
              }
            }
          }

          lifecycle {
            // Pre-stop hook is used to send a fallback signal to the container
            // to gracefully remove all connections ahead of shutdown
            pre_stop {
              exec {
                command = ["sh", "-c", "sleep 5 && echo \"1\" >> shutdown"]
              }
            }
          }

          resources {
            requests = {
              cpu    = local.infra.production ? "350m" : "150m"
              memory = local.infra.production ? "1Gi" : "500Mi"
            }
            limits = {
              cpu    = local.infra.production ? "0.5" : "150m"
              memory = local.infra.production ? "1.5Gi" : "500Mi"
            }
          }

          volume_mount {
            name       = "config"
            mount_path = "/app/config.yaml"
            sub_path   = "config.yaml"
          }

          liveness_probe {
            http_get {
              path = "/"
              port = "health"
            }
            initial_delay_seconds = 10
            timeout_seconds       = 5
            period_seconds        = 5
            success_threshold     = 1
            failure_threshold     = 3
          }

          readiness_probe {
            http_get {
              path = "/"
              port = "health"
            }
            initial_delay_seconds = 3
            timeout_seconds       = 5
            period_seconds        = 5
            success_threshold     = 4
            failure_threshold     = 3
          }

          image_pull_policy = var.image_pull_policy
        }

        volume {
          name = "config"
          secret {
            secret_name = kubernetes_secret.app.metadata[0].name
          }
        }
      }
    }
  }
}

resource "kubernetes_service" "app" {
  metadata {
    name      = "eventapi"
    namespace = kubernetes_namespace.app.metadata[0].name
    labels    = {
      app = "eventapi"
    }
  }

  spec {
    selector = {
      app = "eventapi"
      // "eventapi.k8s.7tv.io/available" = "true"
    }

    port {
      name        = "http"
      port        = 3000
      target_port = "http"
    }

    port {
      name        = "metrics"
      port        = 9100
      target_port = "metrics"
    }

    port {
      name        = "health"
      port        = 9200
      target_port = "health"
    }

    port {
      name        = "pprof"
      port        = 9300
      target_port = "pprof"
    }
  }
}

resource "kubectl_manifest" "app_monitor" {
  depends_on = [kubernetes_deployment.app]

  yaml_body = <<YAML
apiVersion: monitoring.coreos.com/v1
kind: ServiceMonitor
metadata:
  name: eventapi
  namespace: ${kubernetes_namespace.app.metadata[0].name}
  labels:
    app: eventapi
spec:
  selector:
    matchLabels:
      app: eventapi
  endpoints:
    - port: metrics
YAML
}

resource "kubernetes_ingress_v1" "app" {
  metadata {
    name        = "eventapi"
    namespace   = kubernetes_namespace.app.metadata[0].name
    annotations = {
      // "external-dns.alpha.kubernetes.io/target"             = local.infra.cloudflare_tunnel_hostname.longlived
      "external-dns.alpha.kubernetes.io/cloudflare-proxied" = "true"
      "kubernetes.io/ingress.class"                         = "nginx"
    }
  }

  spec {
    rule {
      host = join(".", ["events", local.infra.secondary_zone])
      http {
        path {
          path      = "/"
          path_type = "Prefix"
          backend {
            service {
              name = kubernetes_service.app.metadata[0].name
              port {
                name = "http"
              }
            }
          }
        }
      }
    }

    tls {
      hosts = [ join(".", ["events", local.infra.secondary_zone]) ]
    }
  }
}

resource "kubernetes_horizontal_pod_autoscaler_v2" "app" {
  metadata {
    name      = "eventapi"
    namespace = kubernetes_namespace.app.metadata[0].name
  }

  spec {
    scale_target_ref {
      api_version = "apps/v1"
      kind        = "Deployment"
      name        = kubernetes_deployment.app.metadata[0].name
    }

    min_replicas = local.infra.production ? 4 : 1
    max_replicas = 200

    metric {
      type = "Pods"
      pods {
        metric {
          name = "events_v3_current_connections"
        }

        target {
          type          = "AverageValue"
          average_value = var.connection_limit * 0.75
        }
      }
    }

    metric {
      type = "Resource"
      resource {
        name = "memory"
        target {
          type                = "Utilization"
          average_utilization = 75
        }
      }
    }

    behavior {
      scale_up {
        stabilization_window_seconds = 30
        select_policy                = "Max"

        policy {
          period_seconds = 20
          type           = "Percent"
          value          = 25
        }

        policy {
          period_seconds = 20
          type           = "Pods"
          value          = 4
        }
      }

      scale_down {
        stabilization_window_seconds = 600
        select_policy                = "Max"
        policy {
          period_seconds = 60
          type           = "Percent"
          value          = 5
        }
        policy {
          period_seconds = 60
          type           = "Pods"
          value          = 2
        }
      }
    }
  }
}
