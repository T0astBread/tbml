const port = browser.runtime.connectNative("mothership_native_connector")

function isOnStartPage(tab) {
	return [
		"",
		"about:blank",
		"about:newtab",
		"about:tor",
		"about:torconnect?redirect=about%3Ator",
	].includes(tab.url)
}

port.onMessage.addListener(async msg => {
	console.log("Received:", msg)

	if (typeof msg.data === "object") {
		switch (msg.data.type) {
			case "open-tab":
				const { url } = msg.data
				let openedTab
				if (url && url !== "") {
					const activeTabs = await browser.tabs.query({
						active: true,
					})
					if (activeTabs.length > 0 && isOnStartPage(activeTabs[0])) {
						openedTab = activeTabs[0]
						await browser.tabs.update(openedTab.id, {
							url,
						})
					} else {
						openedTab = await browser.tabs.create({
							url,
						})
					}
				} else {
					openedTab = await browser.tabs.create({})
				}
				await browser.windows.update(openedTab.windowId, {
					focused: true,
				})
				await port.postMessage({
					type: "tbml",
					data: {
						type: "opened-tab",
						url,
					},
				})
		}
	}
})

console.log("Control socket:", controlSocketPath)
port.postMessage({
	type: "init-control-socket-path",
	data: controlSocketPath,
})

port.postMessage({
	type: "tbml",
	data: "Hello from Mothership! :>"
})
