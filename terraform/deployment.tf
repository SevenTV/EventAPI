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
    "config.yaml" = templatefile("${path.module}/config.yaml", {
      redis_address      = var.infra.redis_host,
      redis_password     = var.infra.redis_password,
      bind               = "0.0.0.0:3000",
      heartbeat_interval = var.heartbeat_interval,
      subscription_limit = var.subscription_limit,
      connection_limit   = var.connection_limit,
      ttl                = var.ttl,
    })
  }
}

resource "kubernetes_deployment" "app" {
  metadata {
    name      = "eventapi"
    namespace = kubernetes_namespace.app.metadata[0].name
    labels = {
      app = "eventapi"
    }
  }

  timeouts {
    create = "2m"
    update = "2m"
    delete = "2m"
  }

  spec {
    selector {
      match_labels = {
        app = "eventapi"
      }
    }

    template {
      metadata {
        labels = {
          app = "eventapi"
        }
      }

      spec {
        container {
          name  = "eventapi"
          image = var.app_docker_image

          port "http" {
            name           = "http"
            container_port = 3000
            protocol       = "TCP"
          }

          port "metrics" {
            name           = "metrics"
            container_port = 9100
            protocol       = "TCP"
          }

          port "health" {
            name           = "health"
            container_port = 9200
            protocol       = "TCP"
          }

          port "pprof" {
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
              cpu    = "350m"
              memory = "3000Mi"
            }
            limits = {
              cpu    = "1000m"
              memory = "3225Mi"
            }
          }

          volume_mount {
            name       = "config"
            mount_path = "/app/config.yaml"
            sub_path   = "config.yaml"
          }

          liveness_probe {
            tcp_socket {
              port = "health"
            }
            initial_delay_seconds = 3
            timeout_seconds       = 5
            period_seconds        = 5
            success_threshold     = 1
            failure_threshold     = 6
          }

          readiness_probe {
            tcp_socket {
              port = "health"
            }
            initial_delay_seconds = 3
            timeout_seconds       = 5
            period_seconds        = 5
            success_threshold     = 1
            failure_threshold     = 6
          }

          // TODO: This should be "IfNotPresent", but first requires the image url to use hashes
          image_pull_policy = "Always"
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
  }

  spec {
    selector = {
      app = "eventapi"
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

resource "kubernetes_ingress_v1" "app" {
  metadata {
    name      = "eventapi"
    namespace = kubernetes_namespace.app.metadata[0].name
    annotations = {
      "kubernetes.io/ingress.class"                         = "nginx"
      "external-dns.alpha.kubernetes.io/hostname"           = local.infra.cloudflare_tunnel_hostname
      "external-dns.alpha.kubernetes.io/cloudflare-proxied" = "true"
    }
  }

  spec {
    rule {
      host = local.infra.secondary_zone
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
  }
}

resource "kubernetes_horizontal_pod_autoscaler_v2" "eventapil" {
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

    min_replicas = 1
    max_replicas = 100

    metrics {
      resource {
        name = "events_v3_current_connections"
        target {
          type  = "Value"
          value = var.connection_limit
        }
      }
    }
  }
}
