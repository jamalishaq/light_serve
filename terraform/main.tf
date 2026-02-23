terraform {
  required_providers {
    azurerm = {
      source  = "hashicorp/azurerm"
      version = "=4.1.0"
    }
  }
}

provider "azurerm" {
  features {}
  subscription_id = var.subscription_id
  resource_provider_registrations = "none"
}

data "azurerm_resource_group" "portfolio" {
  name = "portfolio"
}

data "azurerm_container_registry" "jamalportfolio" {
  name                = "jamalportfolio"
  resource_group_name = data.azurerm_resource_group.portfolio.name
}

resource "azurerm_service_plan" "light_serve_plan" {
  name                = "light-serve-plan"
  location            = "canadacentral"
  resource_group_name = data.azurerm_resource_group.portfolio.name

  os_type  = "Linux"
  sku_name = "F1"
}

resource "azurerm_linux_web_app" "light_serve" {
  name                = "light-serve"
  location            = "canadacentral"
  resource_group_name = data.azurerm_resource_group.portfolio.name
  service_plan_id     = azurerm_service_plan.light_serve_plan.id

  site_config {
    always_on = false

    application_stack {
      docker_image_name   = "light-serve:v1"
      docker_registry_url  = "https://${data.azurerm_container_registry.jamalportfolio.login_server}"
    }
  }

  app_settings = {
    "WEBSITES_PORT" = "8080"
    "WEBSITES_ENABLE_APP_SERVICE_STORAGE" = "false"
  }
}