namespace StreamWebRom.Launcher;

public sealed class RelayForm : Form
{
    public RelayForm()
    {
        var s = RelaySettings.Load();
        Text = Lang.T("relay.title"); Width = 700; Height = 600; MinimumSize = new Size(620, 500);
        StartPosition = FormStartPosition.CenterParent;
        var grid = new TableLayoutPanel { Dock = DockStyle.Fill, Padding = new Padding(18), AutoScroll = true, ColumnCount = 2 };
        grid.ColumnStyles.Add(new ColumnStyle(SizeType.Absolute, 220)); grid.ColumnStyles.Add(new ColumnStyle(SizeType.Percent, 100));
        void Row(string label, Control control) { var n = grid.RowCount++; grid.RowStyles.Add(new RowStyle(SizeType.AutoSize)); grid.Controls.Add(new Label { Text = label, AutoSize = true, Margin = new Padding(3, 7, 3, 7) }, 0, n); control.Dock = DockStyle.Top; grid.Controls.Add(control, 1, n); }
        TextBox Field(string value, bool secret = false) => new() { Text = value, UseSystemPasswordChar = secret, PlaceholderText = secret ? Lang.T("relay.emptySecret") : "" };
        var enabled = new CheckBox { Text = Lang.T("relay.enable"), Checked = s.Enabled, AutoSize = true };
        var cloud = new CheckBox { Text = Lang.T("relay.cloudflare"), Checked = s.CloudflareEnabled, AutoSize = true };
        var ip = Field(s.PublicIp); var listen = Field(s.Listen); var alternate = Field(s.TcpAlternate);
        var advertised = Field(s.AdvertisedUrls);
        var account = Field(s.CloudflareAccount); var keyId = Field(s.CloudflareTurnKeyId);
        var key = Field("", true); var analytics = Field("", true);
        var budget = new NumericUpDown { Minimum = 1, Maximum = 900, Value = Math.Clamp((decimal)s.BudgetBytes / 1000000000m, 1, 900) };
        Row(Lang.T("relay.row.localTurn"), enabled); Row(Lang.T("relay.row.publicIp"), ip); Row(Lang.T("relay.row.listen"), listen); Row(Lang.T("relay.row.tcpAlternate"), alternate);
        Row(Lang.T("relay.row.advertised"), advertised);
        Row(Lang.T("relay.row.ports"), new Label { Text = Lang.T("relay.portsHint", s.MinPort, s.MaxPort), AutoSize = true });
        Row(Lang.T("relay.row.provider"), cloud); Row(Lang.T("relay.row.account"), account); Row(Lang.T("relay.row.keyId"), keyId);
        Row(Lang.T("relay.row.keyToken"), key); Row(Lang.T("relay.row.analytics"), analytics); Row(Lang.T("relay.row.budget"), budget);
        Row("", new Label { Text = Lang.T("relay.note"), AutoSize = true, MaximumSize = new Size(400, 0) });
        var save = new Button { Text = Lang.T("relay.save"), AutoSize = true };
        save.Click += (_, _) => {
            if (enabled.Checked && (!System.Net.IPAddress.TryParse(ip.Text.Trim(), out var parsed) || parsed.AddressFamily != System.Net.Sockets.AddressFamily.InterNetwork)) { MessageBox.Show(this, Lang.T("relay.error.invalidIp")); return; }
            if (cloud.Checked && !enabled.Checked) { MessageBox.Show(this, Lang.T("relay.error.cloudflareNeedsLocal")); return; }
            s.Enabled = enabled.Checked; s.CloudflareEnabled = cloud.Checked; s.PublicIp = ip.Text.Trim(); s.Listen = listen.Text.Trim(); s.TcpAlternate = alternate.Text.Trim();
            s.AdvertisedUrls = advertised.Text.Trim();
            s.CloudflareAccount = account.Text.Trim(); s.CloudflareTurnKeyId = keyId.Text.Trim(); s.BudgetBytes = (ulong)(budget.Value * 1000000000m);
            if (key.Text.Length > 0) s.CloudflareTurnKeyProtected = RelaySettings.Encrypt(key.Text.Trim());
            if (analytics.Text.Length > 0) s.CloudflareAnalyticsTokenProtected = RelaySettings.Encrypt(analytics.Text.Trim());
            try { s.Save(); DialogResult = DialogResult.OK; Close(); } catch (Exception ex) { MessageBox.Show(this, ex.Message); }
        };
        Row("", save); Controls.Add(grid); AcceptButton = save;
    }
}
