terraform {
  required_providers {
    azurerm = {
      source  = "hashicorp/azurerm"
      version = "~> 3.0"
    }
  }
}

provider "azurerm" {
  features {}
}

# count-indexed NSG with security rules (Category 3 — multiple sibling blocks)
resource "azurerm_network_security_group" "nsg" {
  count               = 2
  name                = "graft-absorb-${count.index}-nsg"
  location            = "eastus"
  resource_group_name = "graft-test-rg"

  security_rule {
    name                       = "allow-ssh"
    priority                   = 100
    direction                  = "Inbound"
    access                     = "Allow"
    protocol                   = "Tcp"
    source_port_range          = "*"
    destination_port_range     = "22"
    source_address_prefix      = "*"
    destination_address_prefix = "*"
  }

  tags = {
    environment = "test"
    project     = "graft"
  }
}

# for_each-indexed VM with os_disk (Category 2 — single nested block)
resource "azurerm_linux_virtual_machine" "vm" {
  for_each            = toset(["web", "api"])
  name                = "graft-${each.key}-vm"
  location            = "eastus"
  resource_group_name = "graft-test-rg"
  size                = "Standard_DS1_v2"

  admin_username = "adminuser"
  admin_password = "P@ssw0rd1234!"

  network_interface_ids = []

  os_disk {
    caching              = "ReadOnly"
    storage_account_type = "Standard_LRS"
    disk_size_gb         = 30
  }

  source_image_reference {
    publisher = "Canonical"
    offer     = "UbuntuServer"
    sku       = "18.04-LTS"
    version   = "latest"
  }
}
