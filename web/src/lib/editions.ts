export interface EditionStyle {
	border: string;
	bg: string;
	bgRaw: string;
	activeBorder: string;
	activeBg: string;
}

const defaultStyle: EditionStyle = {
	border: 'border-gray-700',
	bg: 'bg-gray-900',
	bgRaw: 'rgba(31, 41, 55, 0.4)',
	activeBorder: 'border-indigo-500',
	activeBg: 'bg-indigo-500/10'
};

const styles: Record<string, EditionStyle> = {
	tb: {
		border: 'border-rose-800/60',
		bg: 'bg-rose-950/40',
		bgRaw: 'rgba(76, 5, 25, 0.4)',
		activeBorder: 'border-rose-500',
		activeBg: 'bg-rose-900/50'
	},
	bmr: {
		border: 'border-orange-800/60',
		bg: 'bg-orange-950/40',
		bgRaw: 'rgba(67, 20, 7, 0.4)',
		activeBorder: 'border-orange-500',
		activeBg: 'bg-orange-900/50'
	},
	snv: {
		border: 'border-violet-700/60',
		bg: 'bg-violet-950/40',
		bgRaw: 'rgba(46, 16, 101, 0.4)',
		activeBorder: 'border-violet-500',
		activeBg: 'bg-violet-900/50'
	}
};

export function editionStyle(id: string): EditionStyle {
	return styles[id] ?? defaultStyle;
}
