import { useEffect, useRef, useState } from "react";
import { Typography } from "antd";
import { searchUsers } from "@/api/users";
import type { User } from "@/api/auth";

/** 用户搜索 AutoComplete 的状态与选项（配合 antd AutoComplete 使用） */
export function useUserSearch(
  value: string,
  onChange: (value: string) => void,
) {
  const [query, setQuery] = useState(value);
  const [results, setResults] = useState<User[]>([]);
  const [loading, setLoading] = useState(false);
  const timerRef = useRef<ReturnType<typeof setTimeout> | undefined>(undefined);
  const usersByUsername = useRef<Map<string, User>>(new Map());
  useEffect(() => {
    if (!value) setQuery("");
  }, [value]);
  async function search(q: string) {
    onChange(q);
    if (q.length < 2) {
      setResults([]);
      usersByUsername.current.clear();
      return;
    }
    setLoading(true);
    try {
      const res = await searchUsers(q);
      setResults(res.users);
      usersByUsername.current = new Map(res.users.map((u) => [u.username, u]));
    } finally {
      setLoading(false);
    }
  }
  function onSearchInput(val: string) {
    setQuery(val);
    clearTimeout(timerRef.current);
    timerRef.current = setTimeout(() => search(val.trim()), 250);
  }
  function pick(username: string, onSelect: (user: User) => void) {
    const user = usersByUsername.current.get(username);
    if (!user) return;
    setQuery(user.username);
    onChange(user.username);
    onSelect(user);
  }
  const options = results.map((u) => ({
    value: u.username,
    label: (
      <div>
        <strong>{u.name || u.username}</strong>
        <Typography.Text type="secondary" style={{ marginLeft: 8 }}>
          @{u.username} · {u.email}
        </Typography.Text>
      </div>
    ),
  }));
  return { query, loading, options, onSearchInput, pick };
}
