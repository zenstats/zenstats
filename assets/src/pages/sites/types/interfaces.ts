export interface Site {
  id: number;
  domain: string;
  remark: string;
  role: string;
  timezone?: string;
}


export interface TimeRangeVisitor {
  date: string;
  uv: number;
}

export interface TopStats {
  pv: number;
  uv: number;
  sessions: number;
  pv_change: number;
  uv_change: number;
  session_change: number;
  avg_duration: number;
  avg_duration_change: number;
  avg_duration_format: string;
  bounce_rate: number;
}

export interface RankItem {
  key: string;
  visits: number;
  percentage?: number;
}

export interface StatsRequest {
  period: string;
  date?: string; // 日期，可选
  from?: string; // 开始日期，可选
  to?: string; // 结束日期，可选
  interval?: Interval;

  refresh?: Date
}

export type Interval = 'hour' | 'day' | 'week' | 'minute';