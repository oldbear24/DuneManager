using DuneManager.Shared;
using DuneManager.WinForms;

ApplicationConfiguration.Initialize();
var configStore = new ConfigStore();
Application.Run(new MainForm(configStore));
