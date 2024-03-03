const MAX_WIDTH = MAX_HEIGHT = 1000,
	WATERMARK_SRC = "/static/res/wm.png",
	WATERMARK_WIDTH = 325,
	WATERMARK_HEIGHT = 98,
	IG_TEMPLATE_SRC = "/static/res/ig.png",
	IG_MIN_OFFSET_X = 0,
	IG_MIN_OFFSET_Y = 342,
	IG_MAX_WIDTH = 1080,
	IG_MAX_HEIGHT = 988,

	WATERMARK = new Image(),
	IG_TEMPLATE = new Image(),
	fanarts = new Array(),
	months = new Set(),
	last_updated_link = document.getElementById("last-updated-link"),
	month_input = document.getElementById("month_input"),
	month_div = document.getElementById("month_div"),
	get_button = document.getElementById("get_button"),
	selectall_button = document.getElementById("selectall_button"),
	selectnone_button = document.getElementById("selectnone_button"),
	controls_div = document.getElementById("controls"),
	opacity_range = document.getElementById("opacity_range"),
	content_div = document.getElementById("content"),
	canvas_link = document.getElementById("canvas-download"),
	canvas_ig = document.getElementById("instagram-canvas"),
	fanart_template = document.getElementById("fanart-template").innerHTML,
	radians_factor = 90 * Math.PI / 180;

let	new_entries = 0;

WATERMARK.src = WATERMARK_SRC;
IG_TEMPLATE.src = IG_TEMPLATE_SRC;
month_input.addEventListener("keyup", (e) => { if (e.key === "Enter") getArtworks(); });

function addCanvasEvents(element, ctx) {
	function abc(e) {
		if (element.enabled == 0) return;
		if (e.button != 0) return;
		element.watermark.opacity = opacity_range.value;
		addWatermark(e, element, ctx);
	}
	if (element.clicked) {
		element.canvas.addEventListener('mousedown', abc);
		return;
	}
	element.canvas.addEventListener('mousemove', abc);
	element.canvas.addEventListener('mouseleave', () => {
		if (!element.clicked && element.enabled)
			setBaseImage(element);
	});
	element.canvas.addEventListener('mousedown', (e) => {
		element.clicked = true;
		element.canvas.removeEventListener('mousemove', abc);
		element.canvas.addEventListener('mousedown', abc);
	});
}

function createElementFromHTML(htmlString) {
	const div = document.createElement('div');
	div.innerHTML = htmlString.trim();
	return div.firstChild;
}

function getNewCardHtml(element) {
	originalFileName = element.filename.split
	const id = element.id,
		index = element.index,
		name = element.name,
		content = element.content,
		text = `${element.name} → “${element.filename}”`,
		disabled = element.enabled == 0 ? " entry-disabled" : "",
		save_filename = `${('0' + index).slice(-2)} - ${name}.png`;
	const html_string = Mustache.render("{{={| |}=}}" + fanart_template, { id, index, name, content, text, disabled, save_filename });
	element.div = createElementFromHTML(html_string);
	element.canvas = element.div.getElementsByTagName("canvas")[0];
	const ctx = element.canvas.getContext("2d");
	element.image = new Image();
	element.image.addEventListener("load", () => {
		const wm_info = element.watermark;
		setBaseImage(element);
		if (element.clicked)
			drawWatermark(wm_info, ctx);
		addCanvasEvents(element, ctx);
	});
	element.image.src = element.content;
	return element.div;
}

async function updateFanartList() {
	content_div.innerHTML = "";
	get_button.disabled = false;
	get_button.innerText = emoji_get;

	let i = 0;
	for (fanart of fanarts) {
		if (fanart.enabled) {
			i++;
			fanart.index = i;
			content_div.appendChild(getNewCardHtml(fanart));
		}
	}
	for (fanart of fanarts) {
		if (!fanart.enabled) {
			fanart.index = 0;
			content_div.appendChild(getNewCardHtml(fanart));
		}
	}
}

function getFanart(id) {
	return fanarts.find(element => element.id == id);
}

function toggleEntry(id) {
	entry = getFanart(id);
	if (!entry) return;

	entry.enabled = !entry.enabled;
	entry.clicked = false;
	updateFanartList();
}

function saveCanvas(my_canvas, filename) {
	canvas_link.href = my_canvas.toDataURL("image/png").replace("image/png", "image/octet-stream");
	canvas_link.setAttribute("download", filename);
	canvas_link.click();
}

function reloadEntry(id) {
	const fanart = getFanart(id);
	if (!fanart) return;

	const old_div = fanart.div;
	content_div.replaceChild(getNewCardHtml(fanart), old_div);
	old_div.remove();
}

function selectAllNone(toggle) {
	fanarts.map(x => x.enabled = toggle)
	updateFanartList();
}

function clickCoordsToCanvas(clickX, clickY, c) {
	const rect = c.getBoundingClientRect();
	const x = (clickX - rect.left) * c.width / c.clientWidth;
	const y = (clickY - rect.top) * c.height / c.clientHeight;
	return [x, y];
}

function drawWatermark(wm_info, ctx) {
	ctx.globalAlpha = wm_info.opacity;
	ctx.filter = wm_info.invert;
	ctx.drawImage(WATERMARK, wm_info.x - (WATERMARK_WIDTH / 2), wm_info.y - (WATERMARK_HEIGHT / 2), WATERMARK_WIDTH, WATERMARK_HEIGHT);
}

function addWatermark(event, element, ctx) {
	setBaseImage(element);
	let [x, y] = clickCoordsToCanvas(event.clientX, event.clientY, element.canvas);
	element.watermark.x = x;
	element.watermark.y = y;
	drawWatermark(element.watermark, ctx);
}

function updateOpacity() {
	opacity_label.innerHTML = Math.round(opacity_range.value * 100) + '%';
}

function getFactor(img_width, img_height, max_width, max_height) {
	return Math.min(max_width / img_width, max_height / img_height);
}

function setBaseImage(element) {
	const img = element.image;
	const c = element.canvas;
	const ctx = c.getContext("2d");
	const f = getFactor(img.width, img.height, MAX_WIDTH, MAX_HEIGHT);
	ctx.imageSmoothingEnabled = (f < 1);

	const pre_width = Math.ceil(img.width * f);
	const pre_height = Math.ceil(img.height * f);

	if (element.rotated % 2) [c.width, c.height] = [pre_height, pre_width];
	else [c.width, c.height] = [pre_width, pre_height];
	
	const rotation_radians = radians_factor * element.rotated;
	ctx.rotate(rotation_radians);

	offset_x = (element.rotated == 2 || element.rotated == 3) ? -pre_width : 0;
	offset_y = (element.rotated == 1 || element.rotated == 2) ? -pre_height : 0;
	ctx.drawImage(img, offset_x, offset_y, pre_width, pre_height);
	ctx.rotate(-rotation_radians);
}

function rotateEntry(id) {
	const entry = fanarts.find(element => element.id == id);
	if (!entry) return;
	if (!entry.enabled) return;

	entry.rotated++;
	if (entry.rotated >= 4 || entry.rotated < 0) entry.rotated = 0;
	entry.clicked = false;
	updateFanartList();
}

function moveUpDown(id, amount) {
	const entry = fanarts.find(element => element.id == id);
	if (!entry) return;
	if (!entry.enabled) return;
	const pos = fanarts.indexOf(entry);
	const new_pos = pos + amount;

	if (new_pos <= -1 || new_pos >= fanarts.length) return;
	[fanarts[pos], fanarts[new_pos]] = [fanarts[new_pos], fanarts[pos]];
	updateFanartList();
}

function toggleInvert(id, button) {
	const entry = fanarts.find(element => element.id == id);
	if (!entry) return;
	if (!entry.enabled) return;
	entry.watermark.invert = entry.watermark.invert == '' ? 'invert(1)' : '';
	button.innerText = entry.watermark.invert ? emoji_color_black : emoji_color;
	const ctx = entry.canvas.getContext('2d');
	setBaseImage(entry);
	drawWatermark(entry.watermark, ctx);
}

async function postData(url = "", data = {}) {
	const response = await fetch(url, { method: "POST", headers: { "Content-Type": "application/x-www-form-urlencoded" }, body: new URLSearchParams(data).toString() });
	return response.json();
}

function saveEntry(id) {
	const entry = getFanart(id);
	if (!entry) return;
	if (!entry.enabled) return;
	saveCanvas(entry.canvas, entry.canvas.getAttribute("data-filename"));
}

function saveCanvasIG(my_canvas) {
	const f = getFactor(my_canvas.width, my_canvas.height, IG_MAX_WIDTH, IG_MAX_HEIGHT);
	const width = Math.ceil(my_canvas.width * f);
	const height = Math.ceil(my_canvas.height * f);
	const offset_x = Math.round((IG_MAX_WIDTH - width) / 2);
	const offset_y = Math.round((IG_MAX_HEIGHT - height) / 2);
	destCtx = canvas_ig.getContext('2d');
	destCtx.drawImage(IG_TEMPLATE, 0, 0);
	destCtx.drawImage(my_canvas, IG_MIN_OFFSET_X + offset_x, IG_MIN_OFFSET_Y + offset_y, width, height);
	saveCanvas(canvas_ig, "IG - " + my_canvas.getAttribute("data-filename"));
}

function saveEntryIG(id) {
	entry = getFanart(id);
	if (!entry) return;
	if (!entry.enabled) return;
	saveCanvasIG(entry.canvas);
}

function getArtworks() {
	const month_value = month_input.value;
	if (months.has(month_value)) return;
	get_button.disabled = true;
	get_button.innerText = "🔄"
	fetch("/get?month=" + month_value).then((res) => {
		res.json().then((data) => {
			fanarts.push(...data.map((element) => {
				return {
					...element,
					"enabled": 1,
					"rotated": 0,
					"watermark": { invert: "" },
				};
			}));
			controls_div.hidden = false;
			updateOpacity();
			updateFanartList();
			months.add(month_value);
		})
	})
}

function saveAll() {
	const response = confirm("Vuoi davvero scaricare tutte le fanart?");
	if (response == false) return;
	fanarts.forEach((fanart) => {
		if (fanart.enabled)
			saveCanvas(fanart.canvas, fanart.canvas.getAttribute("data-filename"));
	})
}

function saveAllIG() {
	const response = confirm("Vuoi davvero scaricare tutte le storie per le fanart?");
	if (response == false) return;
	fanarts.forEach((fanart) => {
		if (fanart.enabled)
			saveCanvasIG(fanart.canvas);
	});
}

function updateDb() {
	postData("/update").then((data) => {
		last_updated_link.innerText = data.timestamp;
		if (data.new == 0) return;
		new_entries += data.new;
		last_updated_link.innerText += ` (+${ new_entries })`;
	});
}

function toClipBoard(id) {
	const fanart = getFanart(id);
	navigator.clipboard.writeText(fanart.canvas.getAttribute("data-filename"));
}
