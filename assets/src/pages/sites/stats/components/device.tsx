import { Card, CardContent, CardHeader, CardTitle } from "@/components/ui/card";
import type { RankItem, StatsRequest } from "@/pages/sites/types/interfaces";
import { Skeleton } from "@components/ui/skeleton";
import {
  useCallback,
  useEffect,
  useState,
  type Dispatch,
  type SetStateAction,
} from "react";
import { type BaseResponse } from "@utils/axios";

interface TimeRangeProps {
  limit?: number;
  query: StatsRequest;
  setQuery?: Dispatch<SetStateAction<StatsRequest>>;
  api: (dateRange: StatsRequest) => Promise<BaseResponse<RankItem[]>>;
}

export default function Devices({
  limit = 5,
  query,
  // setQuery,
  api,
}: TimeRangeProps) {
  const [localLoading, setLocalLoading] = useState(false);
  const [data, setData] = useState<BaseResponse<RankItem[]>>();

  const fetchData = useCallback(async () => {
    try {
      setLocalLoading(true);
      const result = await api(query);
      setData(result);
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
  }, [fetchData, query.refresh]);

  return (
    <Card>
      <CardHeader>
        <CardTitle>Devices</CardTitle>
      </CardHeader>
      <CardContent>
        <div className="h-[300px] flex items-center justify-center">
          {localLoading ? (
            <Skeleton className="h-full w-full rounded-md" />
          ) : (
            <div className="w-full h-full flex flex-col md:flex-row items-center justify-around">
              <div className="w-full h-full flex flex-col justify-center">
                {data?.data?.map(
                  (device, index) =>
                    index <= limit && (
                      <div key={index} className="flex items-center mb-6">
                        <div className="w-4 h-4 rounded-full mr-3"></div>
                        <div className="flex-1">
                          <div className="flex justify-between mb-1">
                            <span className="text-sm font-medium">
                              {device.key}
                            </span>
                            <span className="text-sm font-medium">
                              {device.percentage}%
                            </span>
                          </div>
                          <div className="w-full bg-gray-200 rounded-full h-2">
                            <div
                              className="bg-blue-600 h-2 rounded-full"
                              style={{ width: `${device.percentage}%` }}
                            ></div>
                          </div>
                        </div>
                      </div>
                    ),
                )}
              </div>
            </div>
          )}
        </div>
      </CardContent>
    </Card>
  );
}
