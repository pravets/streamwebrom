package lat.webrom.streamwebrom;

import android.Manifest;
import android.app.Activity;
import android.content.Intent;
import android.content.pm.PackageManager;
import android.net.Uri;
import android.net.http.SslCertificate;
import android.net.http.SslError;
import android.os.Bundle;
import android.os.Build;
import android.view.View;
import android.view.WindowManager;
import android.webkit.*;
import android.widget.*;
import java.util.ArrayList;

/** WebRTC client with opt-in foreground-service support for background audio. */
public final class MainActivity extends Activity {
    private WebView web;
    private LinearLayout root;
    private EditText address;
    private Uri trusted;
    private PermissionRequest pending;
    private ValueCallback<Uri[]> fileCallback;
    private String resumeURL;
    private boolean active;
    private boolean backgroundEnabled;
    private Button backgroundButton;

    @Override public void onCreate(Bundle state) {
        super.onCreate(state);
        getWindow().setFlags(WindowManager.LayoutParams.FLAG_SECURE, WindowManager.LayoutParams.FLAG_SECURE);
        root = new LinearLayout(this);
        root.setOrientation(LinearLayout.VERTICAL);
        // Keep controls clear of system bars, including Android's edge-to-edge mode.
        root.setOnApplyWindowInsetsListener((v, insets) -> {
            v.setPadding(insets.getSystemWindowInsetLeft(), insets.getSystemWindowInsetTop(),
                insets.getSystemWindowInsetRight(), insets.getSystemWindowInsetBottom());
            return insets.consumeSystemWindowInsets();
        });
        LinearLayout bar = new LinearLayout(this);
        address = new EditText(this);
        address.setSingleLine(true);
        address.setHint(getString(R.string.hint_server));
        address.setInputType(android.text.InputType.TYPE_CLASS_TEXT | android.text.InputType.TYPE_TEXT_VARIATION_URI);
        address.setText(getPreferences(0).getString("server", ""));
        bar.addView(address, new LinearLayout.LayoutParams(0, -2, 1));
        Button open = new Button(this); open.setText(getString(R.string.open));
        open.setOnClickListener(v -> open(address.getText().toString().trim()));
        bar.addView(open);
        Button browser = new Button(this); browser.setText(getString(R.string.browser));
        browser.setOnClickListener(v -> {
            String url = web == null ? address.getText().toString().trim() : web.getUrl();
            if (valid(Uri.parse(url == null ? "" : url))) external(Uri.parse(url));
        });
        bar.addView(browser);
        backgroundButton = new Button(this); backgroundButton.setText(getString(R.string.background));
        backgroundButton.setOnClickListener(v -> {
            if (backgroundEnabled) { disableBackground(); return; }
            if (web == null || !sameOrigin(Uri.parse(web.getUrl() == null ? "" : web.getUrl()))) {
                message(getString(R.string.bg_need_room)); return;
            }
            if (checkSelfPermission(Manifest.permission.RECORD_AUDIO) != PackageManager.PERMISSION_GRANTED) {
                message(getString(R.string.bg_need_mic)); return;
            }
            if (Build.VERSION.SDK_INT >= 33 && checkSelfPermission(Manifest.permission.POST_NOTIFICATIONS) != PackageManager.PERMISSION_GRANTED) {
                requestPermissions(new String[]{Manifest.permission.POST_NOTIFICATIONS}, 12); return;
            }
            enableBackground();
        });
        // A separate row keeps the address usable on narrow phones.
        LinearLayout backgroundBar = new LinearLayout(this);
        backgroundBar.addView(backgroundButton);
        root.addView(bar);
        root.addView(backgroundBar);
        TextView help = new TextView(this);
        help.setText(getString(R.string.help));
        help.setPadding(12, 8, 12, 8); root.addView(help);
        setContentView(root);
    }

    private static boolean valid(Uri u) {
        return "https".equalsIgnoreCase(u.getScheme()) && u.getHost() != null
            && u.getUserInfo() == null && !u.toString().contains("\\");
    }
    private static int port(Uri u) { return u.getPort() == -1 ? 443 : u.getPort(); }
    private static String originOf(String url) {
        Uri u = Uri.parse(url == null ? "" : url);
        return u.getScheme() + "://" + u.getHost() + (u.getPort() == -1 ? "" : ":" + u.getPort());
    }
    // SHA-256 of the certificate the user is being asked about: the trust is
    // stored per origin *and* pinned to this exact certificate.
    private static String certPin(SslError error) {
        try {
            SslCertificate cert = error.getCertificate();
            java.security.cert.X509Certificate x = cert == null ? null : cert.getX509Certificate();
            if (x == null) return "";
            byte[] digest = java.security.MessageDigest.getInstance("SHA-256").digest(x.getEncoded());
            StringBuilder out = new StringBuilder(digest.length * 2);
            for (byte b : digest) out.append(String.format("%02x", b));
            return out.toString();
        } catch (Exception e) {
            return ""; // no pin: the user is asked every time, never silently trusted
        }
    }
    private boolean sameOrigin(Uri u) {
        return trusted != null && valid(u) && trusted.getHost().equalsIgnoreCase(u.getHost()) && port(trusted) == port(u);
    }
    private void external(Uri u) {
        try { startActivity(new Intent(Intent.ACTION_VIEW, u)); }
        catch (android.content.ActivityNotFoundException e) { message(getString(R.string.no_browser)); }
    }
    private void open(String url) {
        Uri u = Uri.parse(url);
        if (!valid(u)) { message(getString(R.string.bad_url)); return; }
        disableBackground();
        denyPending();
        if (fileCallback != null) { fileCallback.onReceiveValue(null); fileCallback = null; }
        if (web != null) { ((LinearLayout) web.getParent()).removeView(web); web.destroy(); }
        trusted = u;
        // Never persist invitation/host keys, room passwords or room URLs.
        String origin = new Uri.Builder().scheme("https").encodedAuthority(u.getEncodedAuthority()).build().toString();
        getPreferences(0).edit().putString("server", origin).apply();
        address.setText(origin);
        web = new WebView(this);
        web.getSettings().setJavaScriptEnabled(true);
        web.getSettings().setDomStorageEnabled(true);
        web.getSettings().setAllowFileAccess(false);
        web.getSettings().setAllowContentAccess(false);
        web.getSettings().setMixedContentMode(WebSettings.MIXED_CONTENT_NEVER_ALLOW);
        web.getSettings().setMediaPlaybackRequiresUserGesture(true);
        web.setWebViewClient(new WebViewClient() {
            @Override public boolean shouldOverrideUrlLoading(WebView v, WebResourceRequest r) {
                if (sameOrigin(r.getUrl())) return false;
                if (!r.isForMainFrame() && officialVdo(r.getUrl())) return false;
                if (r.isForMainFrame() && r.hasGesture() && valid(r.getUrl())) external(r.getUrl());
                return true;
            }
            // Untrusted certificates (e.g. the server's LAN self-signed mode) are
            // never bypassed silently: the user confirms per origin, and the
            // trust is pinned to the certificate they confirmed. A different
            // certificate for the same origin (an attacker in the LAN, or a
            // regenerated self-signed cert) asks again instead of proceeding.
            @Override public void onReceivedSslError(WebView v, SslErrorHandler handler, SslError error) {
                String origin = originOf(error.getUrl());
                String pin = certPin(error);
                if (!pin.isEmpty() && pin.equals(getPreferences(0).getString("ssl_trust_" + origin, ""))) {
                    handler.proceed(); return;
                }
                new android.app.AlertDialog.Builder(MainActivity.this)
                    .setTitle(R.string.ssl_title)
                    .setMessage(getString(R.string.ssl_message, origin))
                    .setPositiveButton(R.string.ssl_continue, (d, w) -> {
                        if (!pin.isEmpty()) {
                            getPreferences(0).edit().putString("ssl_trust_" + origin, pin).apply();
                        }
                        handler.proceed();
                    })
                    .setNegativeButton(R.string.ssl_cancel, (d, w) -> handler.cancel())
                    .setOnCancelListener(d -> handler.cancel())
                    .show();
            }
            @Override public void onPageFinished(WebView v, String url) {
                getWindow().addFlags(WindowManager.LayoutParams.FLAG_KEEP_SCREEN_ON);
            }
        });
        web.setWebChromeClient(new WebChromeClient() {
            @Override public void onPermissionRequest(PermissionRequest r) {
                runOnUiThread(() -> requestMedia(r));
            }
            @Override public void onPermissionRequestCanceled(PermissionRequest r) {
                runOnUiThread(() -> { if (pending == r) pending = null; });
            }
            @Override public boolean onShowFileChooser(WebView v, ValueCallback<Uri[]> cb, FileChooserParams params) {
                if (!sameOrigin(Uri.parse(v.getUrl() == null ? "" : v.getUrl()))) return false;
                if (fileCallback != null) fileCallback.onReceiveValue(null);
                fileCallback = cb;
                Intent pick = new Intent(Intent.ACTION_GET_CONTENT).addCategory(Intent.CATEGORY_OPENABLE).setType("*/*");
                pick.putExtra(Intent.EXTRA_MIME_TYPES, new String[]{"image/*", "video/*"});
                try { startActivityForResult(pick, 11); }
                catch (android.content.ActivityNotFoundException e) { fileCallback = null; cb.onReceiveValue(null); }
                return true;
            }
        });
        root.addView(web, new LinearLayout.LayoutParams(-1, 0, 1));
        web.loadUrl(url);
    }
    private void requestMedia(PermissionRequest r) {
        if (!active || !permittedMediaOrigin(r.getOrigin())) { r.deny(); return; }
        denyPending(); pending = r;
        ArrayList<String> permissions = new ArrayList<>();
        for (String resource : r.getResources()) {
            String permission = resource.equals(PermissionRequest.RESOURCE_VIDEO_CAPTURE) ? Manifest.permission.CAMERA
                : resource.equals(PermissionRequest.RESOURCE_AUDIO_CAPTURE) ? Manifest.permission.RECORD_AUDIO : null;
            if (permission == null) { denyPending(); return; }
            if (checkSelfPermission(permission) != PackageManager.PERMISSION_GRANTED) permissions.add(permission);
        }
        if (permissions.isEmpty()) grantPending();
        else requestPermissions(permissions.toArray(new String[0]), 10);
    }
    private void grantPending() {
        PermissionRequest r = pending; pending = null;
        if (r == null) return;
        if (!active || !permittedMediaOrigin(r.getOrigin())) { r.deny(); return; }
        ArrayList<String> allowed = new ArrayList<>();
        for (String resource : r.getResources()) {
            if (resource.equals(PermissionRequest.RESOURCE_VIDEO_CAPTURE) && checkSelfPermission(Manifest.permission.CAMERA) == PackageManager.PERMISSION_GRANTED) allowed.add(resource);
            if (resource.equals(PermissionRequest.RESOURCE_AUDIO_CAPTURE) && checkSelfPermission(Manifest.permission.RECORD_AUDIO) == PackageManager.PERMISSION_GRANTED) allowed.add(resource);
        }
        if (allowed.size() == r.getResources().length) r.grant(allowed.toArray(new String[0])); else r.deny();
    }
    private void denyPending() { if (pending != null) { pending.deny(); pending = null; } }
    private boolean officialVdo(Uri u) {
        return "https".equalsIgnoreCase(u.getScheme()) && "vdo.ninja".equalsIgnoreCase(u.getHost())
            && (u.getPort() == -1 || u.getPort() == 443) && u.getUserInfo() == null;
    }
    private boolean permittedMediaOrigin(Uri u) {
        if (sameOrigin(u)) return true;
        Uri top = web == null || web.getUrl() == null ? null : Uri.parse(web.getUrl());
        return officialVdo(u) && top != null && sameOrigin(top) && "/vdo".equals(top.getPath());
    }
    @Override public void onRequestPermissionsResult(int code, String[] permissions, int[] results) {
        super.onRequestPermissionsResult(code, permissions, results);
        if (code == 10) grantPending();
        if (code == 12 && results.length > 0 && results[0] == PackageManager.PERMISSION_GRANTED) enableBackground();
    }
    @Override protected void onActivityResult(int code, int result, Intent data) {
        super.onActivityResult(code, result, data);
        if (code == 11 && fileCallback != null) {
            fileCallback.onReceiveValue(result == RESULT_OK && data != null && data.getData() != null ? new Uri[]{data.getData()} : null);
            fileCallback = null;
        }
    }
    @Override protected void onStart() {
        super.onStart(); active = true;
        if (backgroundEnabled && !CallAudioService.running) disableBackground();
        if (web != null && resumeURL != null) { web.loadUrl(resumeURL); resumeURL = null; message(getString(R.string.restart_notice)); }
    }
    @Override protected void onStop() {
        active = false; denyPending();
        if (web != null && fileCallback == null && !backgroundEnabled) { resumeURL = web.getUrl(); web.loadUrl("about:blank"); }
        getWindow().clearFlags(WindowManager.LayoutParams.FLAG_KEEP_SCREEN_ON);
        super.onStop();
    }
    @Override public void onBackPressed() {
        if (web != null && web.canGoBack()) web.goBack(); else super.onBackPressed();
    }
    @Override protected void onDestroy() {
        disableBackground();
        denyPending(); if (fileCallback != null) fileCallback.onReceiveValue(null);
        if (web != null) web.destroy(); super.onDestroy();
    }
    private void message(String text) { Toast.makeText(this, text, Toast.LENGTH_LONG).show(); }
    private void enableBackground() {
        if (!active || web == null || !sameOrigin(Uri.parse(web.getUrl() == null ? "" : web.getUrl()))) return;
        if (checkSelfPermission(Manifest.permission.RECORD_AUDIO) != PackageManager.PERMISSION_GRANTED) return;
        try {
            Intent service = new Intent(this, CallAudioService.class);
            if (Build.VERSION.SDK_INT >= 26) startForegroundService(service); else startService(service);
            backgroundEnabled = true; backgroundButton.setText(getString(R.string.background_on));
        } catch (IllegalStateException | SecurityException e) {
            disableBackground(); message(getString(R.string.bg_failed));
        }
    }
    private void disableBackground() {
        backgroundEnabled = false;
        stopService(new Intent(this, CallAudioService.class));
        if (backgroundButton != null) backgroundButton.setText(getString(R.string.background));
    }
}
