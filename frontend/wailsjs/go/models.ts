export namespace main {
	
	export class Episode {
	    id: string;
	    number: string;
	    title: string;
	    image: string;
	    duration: string;
	    sub: string;
	    dub: string;
	
	    static createFrom(source: any = {}) {
	        return new Episode(source);
	    }
	
	    constructor(source: any = {}) {
	        if ('string' === typeof source) source = JSON.parse(source);
	        this.id = source["id"];
	        this.number = source["number"];
	        this.title = source["title"];
	        this.image = source["image"];
	        this.duration = source["duration"];
	        this.sub = source["sub"];
	        this.dub = source["dub"];
	    }
	}
	export class Anime {
	    id: number;
	    title: string;
	    image: string;
	    score: string;
	    genres: string[];
	    status: string;
	    episodes: string;
	    synopsis: string;
	    aired: string;
	    studios: string;
	    type: string;
	    duration: string;
	    rating: string;
	    source: string;
	    episodeList: Episode[];
	
	    static createFrom(source: any = {}) {
	        return new Anime(source);
	    }
	
	    constructor(source: any = {}) {
	        if ('string' === typeof source) source = JSON.parse(source);
	        this.id = source["id"];
	        this.title = source["title"];
	        this.image = source["image"];
	        this.score = source["score"];
	        this.genres = source["genres"];
	        this.status = source["status"];
	        this.episodes = source["episodes"];
	        this.synopsis = source["synopsis"];
	        this.aired = source["aired"];
	        this.studios = source["studios"];
	        this.type = source["type"];
	        this.duration = source["duration"];
	        this.rating = source["rating"];
	        this.source = source["source"];
	        this.episodeList = this.convertValues(source["episodeList"], Episode);
	    }
	
		convertValues(a: any, classs: any, asMap: boolean = false): any {
		    if (!a) {
		        return a;
		    }
		    if (a.slice && a.map) {
		        return (a as any[]).map(elem => this.convertValues(elem, classs));
		    } else if ("object" === typeof a) {
		        if (asMap) {
		            for (const key of Object.keys(a)) {
		                a[key] = new classs(a[key]);
		            }
		            return a;
		        }
		        return new classs(a);
		    }
		    return a;
		}
	}
	
	export class LibraryAnime {
	    id: number;
	    animeId: string;
	    title: string;
	    image: string;
	    status: string;
	    score: number;
	    episodesWatch: number;
	    totalEpisodes: string;
	    updatedAt: string;
	
	    static createFrom(source: any = {}) {
	        return new LibraryAnime(source);
	    }
	
	    constructor(source: any = {}) {
	        if ('string' === typeof source) source = JSON.parse(source);
	        this.id = source["id"];
	        this.animeId = source["animeId"];
	        this.title = source["title"];
	        this.image = source["image"];
	        this.status = source["status"];
	        this.score = source["score"];
	        this.episodesWatch = source["episodesWatch"];
	        this.totalEpisodes = source["totalEpisodes"];
	        this.updatedAt = source["updatedAt"];
	    }
	}
	export class SearchResult {
	    id: string;
	    title: string;
	    image: string;
	    score: string;
	    type: string;
	    epsCount: string;
	    status: string;
	    rank?: number;
	    nextEp?: string;
	    nextTime?: string;
	
	    static createFrom(source: any = {}) {
	        return new SearchResult(source);
	    }
	
	    constructor(source: any = {}) {
	        if ('string' === typeof source) source = JSON.parse(source);
	        this.id = source["id"];
	        this.title = source["title"];
	        this.image = source["image"];
	        this.score = source["score"];
	        this.type = source["type"];
	        this.epsCount = source["epsCount"];
	        this.status = source["status"];
	        this.rank = source["rank"];
	        this.nextEp = source["nextEp"];
	        this.nextTime = source["nextTime"];
	    }
	}
	export class StreamLink {
	    url: string;
	    quality: string;
	
	    static createFrom(source: any = {}) {
	        return new StreamLink(source);
	    }
	
	    constructor(source: any = {}) {
	        if ('string' === typeof source) source = JSON.parse(source);
	        this.url = source["url"];
	        this.quality = source["quality"];
	    }
	}
	export class StreamSource {
	    server: string;
	    type: string;
	    links: StreamLink[];
	
	    static createFrom(source: any = {}) {
	        return new StreamSource(source);
	    }
	
	    constructor(source: any = {}) {
	        if ('string' === typeof source) source = JSON.parse(source);
	        this.server = source["server"];
	        this.type = source["type"];
	        this.links = this.convertValues(source["links"], StreamLink);
	    }
	
		convertValues(a: any, classs: any, asMap: boolean = false): any {
		    if (!a) {
		        return a;
		    }
		    if (a.slice && a.map) {
		        return (a as any[]).map(elem => this.convertValues(elem, classs));
		    } else if ("object" === typeof a) {
		        if (asMap) {
		            for (const key of Object.keys(a)) {
		                a[key] = new classs(a[key]);
		            }
		            return a;
		        }
		        return new classs(a);
		    }
		    return a;
		}
	}
	export class TrendingAnime {
	    id: string;
	    title: string;
	    image: string;
	    rank: string;
	    score: string;
	    subs: string;
	    dubs: string;
	    type: string;
	    eps: string;
	
	    static createFrom(source: any = {}) {
	        return new TrendingAnime(source);
	    }
	
	    constructor(source: any = {}) {
	        if ('string' === typeof source) source = JSON.parse(source);
	        this.id = source["id"];
	        this.title = source["title"];
	        this.image = source["image"];
	        this.rank = source["rank"];
	        this.score = source["score"];
	        this.subs = source["subs"];
	        this.dubs = source["dubs"];
	        this.type = source["type"];
	        this.eps = source["eps"];
	    }
	}

}

