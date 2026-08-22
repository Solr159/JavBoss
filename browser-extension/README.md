# JavBoss 助手

This unpacked Manifest V3 extension lets the JavBoss manual scrape dialog open
JavBus, JavLibrary, JavDB, or AVSOX in a new Chrome tab, extract the selected movie
page, and fill the existing manual metadata form. Metadata is never saved automatically;
review it in JavBoss and click the manual scrape button to persist it.

## Install

1. Open `chrome://extensions` in Chrome.
2. Enable **Developer mode**.
3. Click **Load unpacked**.
4. Select this `browser-extension` directory.
5. Reload the JavBoss page after installing or updating the extension.

If upgrading from the earlier iframe-based prototype, remove the old unpacked
extension first and load this directory again. Version 0.2 and later use a fixed
extension ID required by the hidden bridge.

## Use

1. Open a video's scrape settings and select **Manual Scrape**.
2. Click **Open JavBus**, **Open JavLibrary**, **Open JavDB**, or **Open AVSOX** under **Browser extension-assisted scrape**.
3. Complete any site verification and navigate to a movie detail page.
4. Click the injected **Fill JavBoss** button in the lower-right corner.
5. Continue browsing and fill other detail pages as needed.
6. Review the latest filled fields in JavBoss and save them.

The extension-created tab receives a temporary scrape marker in its
`sessionStorage`. A metadata site may open a same-origin child tab, which receives
a copy of the temporary marker. Tabs created manually do not receive it. Filling
metadata keeps the tab and marker active so another detail page can be filled.

JavLibrary opens its Traditional Chinese home page when the code field is empty,
and its parsed metadata defaults to censored unless the page explicitly marks the
movie as uncensored.

JavDB opens its home page when the code field is empty. With a code it opens the
all-category search results page.

AVSOX opens its Traditional Chinese home page when the code field is empty. With
a code it opens the corresponding search page, and metadata is always filled as
uncensored.

JavBoss embeds only the extension's invisible `bridge.html` resource. The metadata
sites are never embedded in JavBoss and cannot access the JavBoss window or login
cookie. The bridge accepts connections only from localhost,
private-network, `.local`, or single-label intranet origins.

The public key in `manifest.json` gives unpacked installations a stable extension
ID. If it is changed, update the matching ID in
`VideoScrapeSettingsModal.jsx`.
