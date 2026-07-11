import type { Equity } from "./types";
import { peerGroup } from "./peerData";

export function exportEquitiesCSV(equities: Equity[]) {
	const rows = equities.map((equity) => ({
		ticker: equity.ticker, company: equity.company, peerGroup: peerGroup(equity), price: equity.current.price, priceDate: equity.current.priceAsOf,
		fundamentalsDate: equity.valuation?.asOf, pe: equity.valuation?.pe, evEbitda: equity.valuation?.evToEbitda, evEbit: equity.valuation?.evToEbit,
		ocfYield: equity.valuation?.operatingCashToMarketCap, fcfYield: equity.valuation?.fcfToMarketCap, netDebtEbitda: equity.valuation?.netDebtToEbitda,
		cashConversion: equity.quality?.cashConversion, grossMargin: equity.quality?.grossMargin, operatingMargin: equity.quality?.operatingMargin,
		ocfMargin: equity.quality?.operatingCashMargin, fcfMargin: equity.quality?.fcfMargin, roic: equity.quality?.roic, incrementalRoic: equity.quality?.incrementalRoic,
		cashConversionCycleDays: equity.quality?.cashConversionCycleDays, stockCompRevenue: equity.quality?.stockCompToRevenue, dilutedShareGrowth: equity.quality?.dilutedShareGrowth,
	}));
	const keys = Object.keys(rows[0] ?? {});
	const csv = [keys.join(","), ...rows.map((row) => keys.map((key) => csvCell(row[key as keyof typeof row])).join(","))].join("\n");
	download(new Blob([csv], { type: "text/csv;charset=utf-8" }), `equities-${new Date().toISOString().slice(0, 10)}.csv`);
}

export async function exportPrimaryChartPNG() {
	const svg = document.querySelector<SVGSVGElement>(".chart-primary svg.recharts-surface, .chart-primary svg");
	if (!svg) throw new Error("No active chart is available to export");
	const clone = svg.cloneNode(true) as SVGSVGElement;
	clone.setAttribute("xmlns", "http://www.w3.org/2000/svg");
	const bounds = svg.getBoundingClientRect();
	clone.setAttribute("width", String(Math.max(1, bounds.width)));
	clone.setAttribute("height", String(Math.max(1, bounds.height)));
	const source = new Blob([new XMLSerializer().serializeToString(clone)], { type: "image/svg+xml;charset=utf-8" });
	const url = URL.createObjectURL(source);
	try {
		const image = new Image();
		await new Promise<void>((resolve, reject) => { image.onload = () => resolve(); image.onerror = () => reject(new Error("Chart image could not be rendered")); image.src = url; });
		const scale = Math.min(2, window.devicePixelRatio || 1);
		const canvas = document.createElement("canvas");
		canvas.width = Math.ceil(bounds.width * scale); canvas.height = Math.ceil(bounds.height * scale);
		const context = canvas.getContext("2d");
		if (!context) throw new Error("Canvas export is unavailable");
		context.scale(scale, scale); context.fillStyle = "#ffffff"; context.fillRect(0, 0, bounds.width, bounds.height); context.drawImage(image, 0, 0, bounds.width, bounds.height);
		const png = await new Promise<Blob>((resolve, reject) => canvas.toBlob((blob) => blob ? resolve(blob) : reject(new Error("PNG encoding failed")), "image/png"));
		download(png, `equity-chart-${new Date().toISOString().slice(0, 10)}.png`);
	} finally { URL.revokeObjectURL(url); }
}

export async function copyCurrentLink() { await navigator.clipboard.writeText(window.location.href); }

function csvCell(value: unknown) {
	if (value === undefined || value === null) return "";
	const text = String(value);
	return /[",\n]/.test(text) ? `"${text.replaceAll('"', '""')}"` : text;
}

function download(blob: Blob, name: string) {
	const url = URL.createObjectURL(blob);
	const anchor = document.createElement("a"); anchor.href = url; anchor.download = name; anchor.click();
	window.setTimeout(() => URL.revokeObjectURL(url), 1000);
}
