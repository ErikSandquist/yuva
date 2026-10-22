-- +goose Up
ALTER TABLE discussion_posts ADD COLUMN via text CHECK (length(via) BETWEEN 1 AND 200);

CREATE OR REPLACE VIEW discussion_post_view AS
SELECT p.id, p.workspace_id, p.discussion_id, p.parent_post_id, p.opening, p.author_type, p.author_member_id,
       p.author_contact_id, p.body, p.state, p.created_at, p.edited_at, p.deleted_at,
       (SELECT count(*) FROM discussion_votes v WHERE v.workspace_id = p.workspace_id AND v.post_id = p.id)::integer AS votes,
       coalesce(CASE
           WHEN p.author_type = 'member' THEN mp.name
           WHEN c.person_id IS NOT NULL THEN cp.name
           ELSE c.name
       END, '')::text AS author_name,
       c.person_id AS author_person_id,
       p.via
FROM discussion_posts p
LEFT JOIN members m ON m.workspace_id = p.workspace_id AND m.id = p.author_member_id
LEFT JOIN people mp ON mp.id = m.person_id
LEFT JOIN contacts c ON c.workspace_id = p.workspace_id AND c.id = p.author_contact_id
LEFT JOIN people cp ON cp.id = c.person_id;

-- +goose Down
DROP VIEW discussion_post_view;
CREATE VIEW discussion_post_view AS
SELECT p.id, p.workspace_id, p.discussion_id, p.parent_post_id, p.opening, p.author_type, p.author_member_id,
       p.author_contact_id, p.body, p.state, p.created_at, p.edited_at, p.deleted_at,
       (SELECT count(*) FROM discussion_votes v WHERE v.workspace_id = p.workspace_id AND v.post_id = p.id)::integer AS votes,
       coalesce(CASE
           WHEN p.author_type = 'member' THEN mp.name
           WHEN c.person_id IS NOT NULL THEN cp.name
           ELSE c.name
       END, '')::text AS author_name,
       c.person_id AS author_person_id
FROM discussion_posts p
LEFT JOIN members m ON m.workspace_id = p.workspace_id AND m.id = p.author_member_id
LEFT JOIN people mp ON mp.id = m.person_id
LEFT JOIN contacts c ON c.workspace_id = p.workspace_id AND c.id = p.author_contact_id
LEFT JOIN people cp ON cp.id = c.person_id;
ALTER TABLE discussion_posts DROP COLUMN via;
