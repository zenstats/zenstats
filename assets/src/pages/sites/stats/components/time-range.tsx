import React, {
  useCallback,
  useEffect,
  useState,
  type Dispatch,
  type SetStateAction,
} from "react";
import {
  Card,
  CardContent,
  CardDescription,
  CardHeader,
  CardTitle,
} from "@/components/ui/card";
import {
  DropdownMenu,
  DropdownMenuTrigger,
  DropdownMenuContent,
  DropdownMenuItem,
} from "@/components/ui/dropdown-menu";
import { Skeleton } from "@/components/ui/skeleton";
import { ChevronDown } from "lucide-react";
import {
  XAxis,
  YAxis,
  CartesianGrid,
  Tooltip,
  ResponsiveContainer,
  AreaChart,
  Area,
} from "recharts";
import type { BaseResponse } from "@utils/axios";
import type {
  StatsRequest,
  TimeRangeVisitor,
} from "@/pages/sites/types/interfaces";

interface TimeRangeProps {
  query: StatsRequest;
  setQuery?: Dispatch<SetStateAction<StatsRequest>>;
  api: (dateRange: StatsRequest) => Promise<BaseResponse<TimeRangeVisitor[]>>;
}

export default function TimeRange({
  query,
  // setQuery,
  api,
}: TimeRangeProps) {
  const [localLoading, setLocalLoading] = useState(false);
  const [data, setData] = useState<BaseResponse<TimeRangeVisitor[]>>();

  const fetchData = useCallback(async () => {
    try {
      setLocalLoading(true);
      const result = await api(query);
      if (result.data && result.data.length > 0) {
        setData(result);
      }
    } catch (error) {
      console.error(error);
    } finally {
      setLocalLoading(false);
    }
  }, [api, query]);

  useEffect(() => {
    if (query.refresh !== undefined) {
      fetchData();
    }
    console.log("refresh time range");
  }, [fetchData, query.refresh]);

  return (
    <Card>
      <CardHeader className="flex flex-row items-center justify-between space-y-0 pb-2">
        <div>
          <CardTitle>访问趋势</CardTitle>
          <CardDescription>今日各时段UV访问量</CardDescription>
        </div>
        <DropdownMenu>
          <DropdownMenuTrigger asChild>
            <button
              type="button"
              className="flex justify-between items-center space-x-2 bg-white p-2 rounded-lg shadow-sm w-[90px] text-left transition-colors hover:bg-gray-50 focus:ring-2 focus:ring-blue-500/20"
            >
              <span className="text-sm font-medium">Hour</span>
              <ChevronDown className="h-4 w-4" />
            </button>
          </DropdownMenuTrigger>
          <DropdownMenuContent className="p-2 rounded-lg">
            <DropdownMenuItem className="flex items-center space-x-3">
              <span>今日</span>
            </DropdownMenuItem>
          </DropdownMenuContent>
        </DropdownMenu>
      </CardHeader>
      <CardContent>
        <div className="h-[300px]">
          {localLoading ? (
            <Skeleton className="h-full w-full rounded-md" />
          ) : (
            <ResponsiveContainer width="100%" height="100%">
              <AreaChart
                data={data?.data}
                margin={{ top: 10, right: 30, left: 0, bottom: 0 }}
              >
                <defs>
                  <linearGradient id="colorUv" x1="0" y1="0" x2="0" y2="1">
                    <stop offset="5%" stopColor="#3b82f6" stopOpacity={0.8} />
                    <stop offset="95%" stopColor="#3b82f6" stopOpacity={0} />
                  </linearGradient>
                </defs>
                <CartesianGrid
                  strokeDasharray="3 3"
                  vertical={false}
                  stroke="#f1f5f9"
                />
                <XAxis
                  dataKey="time"
                  tick={{ fontSize: 12 }}
                  axisLine={false}
                  tickLine={false}
                />
                <YAxis
                  tick={{ fontSize: 12 }}
                  axisLine={false}
                  tickLine={false}
                />
                <Tooltip
                  contentStyle={{
                    borderRadius: "8px",
                    border: "none",
                    boxShadow: "0 4px 12px rgba(0,0,0,0.1)",
                  }}
                  formatter={(value) => [`${value}`, "访问量"]}
                />
                <Area
                  type="monotone"
                  dataKey="uv"
                  stroke="#3b82f6"
                  fillOpacity={1}
                  fill="url(#colorUv)"
                />
              </AreaChart>
            </ResponsiveContainer>
          )}
        </div>
      </CardContent>
    </Card>
  );
}
