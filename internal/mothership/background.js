const port = browser.runtime.connectNative("mothership_native_connector")

port.onMessage.addListener(msg => {
	console.log("Received:", msg)
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
