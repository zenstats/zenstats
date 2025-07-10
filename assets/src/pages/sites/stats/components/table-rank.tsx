import { Card, CardContent, CardHeader, CardTitle } from "@/components/ui/card";
import {
  Table,
  TableBody,
  TableCell,
  TableHead,
  TableHeader,
  TableRow,
} from "@/components/ui/table";
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
  title: string;
  keyName: string;
  limit?: number;
  query: StatsRequest;
  setQuery?: Dispatch<SetStateAction<StatsRequest>>;
  api: (dateRange: StatsRequest) => Promise<BaseResponse<RankItem[]>>;
}

export default function TableRank({
  title,
  keyName,
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
        <CardTitle>{title}</CardTitle>
      </CardHeader>
      <CardContent>
        {localLoading ? (
          <div className="space-y-4">
            {[1, 2, 3, 4, 5].map((i) => (
              <div key={i} className="flex justify-between items-center">
                <Skeleton className="h-5 w-3/5" />
                <Skeleton className="h-5 w-1/5" />
              </div>
            ))}
          </div>
        ) : (
          <Table>
            <TableHeader>
              <TableRow>
                <TableHead>{keyName}</TableHead>
                <TableHead className="text-right">Visitor</TableHead>
                <TableHead className="text-right">Proportion</TableHead>
              </TableRow>
            </TableHeader>
            <TableBody>
              {data?.data?.map(
                (page, index) =>
                  index <= limit && (
                    <TableRow key={index}>
                      <TableCell className="font-medium">{page.key}</TableCell>
                      <TableCell className="text-right">
                        {page.visits.toLocaleString()}
                      </TableCell>
                      <TableCell className="text-right">
                        {page.percentage}%
                      </TableCell>
                    </TableRow>
                  ),
              )}
            </TableBody>
          </Table>
        )}
      </CardContent>
    </Card>
  );
}
