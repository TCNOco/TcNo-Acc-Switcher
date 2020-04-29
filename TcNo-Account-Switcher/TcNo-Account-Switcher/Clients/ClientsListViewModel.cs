using System;
using System.Collections.Generic;
using System.Collections.ObjectModel;
using System.ComponentModel;
using System.Linq;
using System.Text;
using System.Threading.Tasks;

using TcNo_Account_Switcher.Services.Clients;

namespace TcNo_Account_Switcher.Clients
{
    class ClientsListViewModel
    {
        private IClientRepository _clientRepository = new ClientRepository();

        public ClientsListViewModel()
        {
            if (DesignerProperties.GetIsInDesignMode(
                new System.Windows.DependencyObject())) return;

            SoftwareClients = new ObservableCollection<SoftwareClient>(_clientRepository.GetSoftwareClientsAsync().Result);
        }
        public ObservableCollection<SoftwareClient> SoftwareClients { get; set; }
    }
}
