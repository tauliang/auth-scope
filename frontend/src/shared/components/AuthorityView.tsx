import { Ban, Database, KeyRound } from "lucide-react";
import type { AuthorityRegion } from "../api/types";

export function AuthorityView({ authority }: { authority: AuthorityRegion }) {
  return (
    <div className="authority-view">
      <div className="authority-list">
        {(authority.resources ?? []).map((grant, index) => (
          <div className="authority-row" key={`${grant.type}:${grant.id}:${index}`}>
            <Database size={18} aria-hidden="true" />
            <div className="authority-resource">
              <strong>{grant.id}</strong>
              <span>{grant.type}</span>
            </div>
            <div className="action-list">
              {grant.actions.map((action) => <span className="action-chip" key={action}><KeyRound size={12} />{action}</span>)}
            </div>
            {grant.constraints && Object.keys(grant.constraints).length ? <code>{JSON.stringify(grant.constraints)}</code> : null}
          </div>
        ))}
      </div>
      {(authority.forbidden_actions?.length ?? 0) > 0 ? (
        <div className="forbidden-band">
          <Ban size={17} aria-hidden="true" />
          <strong>Forbidden</strong>
          {authority.forbidden_actions?.map((action) => <span key={action}>{action}</span>)}
        </div>
      ) : null}
    </div>
  );
}
